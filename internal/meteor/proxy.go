package meteor

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/things-go/go-socks5"
)

type Proxy struct {
	Protocol string    `yaml:"protocol"`
	Addr     string    `yaml:"addr"`
	Cert     string    `yaml:"cert"`
	Key      string    `yaml:"key"`
	Auth     bool      `yaml:"auth"`
	Accounts []Account `yaml:"accounts"`
}

type Account struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func (p Proxy) Run(ctx context.Context) {
	switch p.Protocol {
	case "http", "https":
		p.startHTTPProxyServer(ctx)
	case "socks5":
		p.startSocks5ProxyServer(ctx)
	}
}

func (p Proxy) startHTTPProxyServer(ctx context.Context) {
	server := &http.Server{
		Addr:              preprocessingAddr(p.Addr),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
		BaseContext:       func(net.Listener) context.Context { return ctx },
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p.Auth && !p.auth(r) {
				p.writeBasicUnauthorized(w)
				return
			}
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else {
				handleHttp(w, r)
			}
		}),
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		var err error
		if p.Protocol == "https" {
			err = server.ListenAndServeTLS(p.Cert, p.Key)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("proxy server failed", "protocol", p.Protocol, "addr", p.Addr, "error", err)
		}
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		slog.Error("proxy shutdown failed", "protocol", p.Protocol, "addr", p.Addr, "error", err)
	}
	<-done
}

func (p Proxy) writeBasicUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Proxy-Authenticate", `Basic realm="Restricted"`)
	w.WriteHeader(http.StatusProxyAuthRequired)
	_, _ = fmt.Fprint(w, http.StatusText(http.StatusProxyAuthRequired))
}

func (p Proxy) auth(r *http.Request) bool {
	credentials := &http.Request{Header: make(http.Header)}
	credentials.Header.Set("Authorization", r.Header.Get("Proxy-Authorization"))
	username, password, ok := credentials.BasicAuth()
	return ok && p.Valid(username, password, "")
}

func (p Proxy) Valid(username, password, _ string) bool {
	if !p.Auth {
		return true
	}
	var passed = false
	for _, account := range p.Accounts {
		if subtle.ConstantTimeCompare([]byte(account.Username), []byte(username)) == 1 && subtle.ConstantTimeCompare([]byte(account.Password), []byte(password)) == 1 {
			passed = true
			break
		}
	}
	return passed
}

func (p Proxy) startSocks5ProxyServer(ctx context.Context) {

	options := []socks5.Option{socks5.WithRule(&socks5.PermitCommand{EnableConnect: true}), socks5.WithDial(func(_ context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Duration(Timeout) * time.Second}).DialContext(ctx, network, addr)
	})}
	if p.Auth {
		options = append(options, socks5.WithCredential(p))
	}
	server := socks5.NewServer(options...)

	// 指定监听地址和端口
	addr := preprocessingAddr(p.Addr)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("SOCKS5 listen failed", "addr", addr, "error", err)
		return
	}

	defer ln.Close()
	stop := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stop()
	var workers sync.WaitGroup
	defer workers.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("SOCKS5 accept failed", "addr", addr, "error", err)
			}
			return
		}
		workers.Go(func() {
			defer conn.Close()
			stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
			defer stop()
			if err := server.ServeConn(conn); err != nil && ctx.Err() == nil {
				slog.Debug("SOCKS5 client failed", "remote", conn.RemoteAddr(), "error", err)
			}
		})
	}

}

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}
	dialer := &net.Dialer{Timeout: time.Duration(Timeout) * time.Second, KeepAlive: 30 * time.Second}
	remote, err := dialer.DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, "upstream connection failed", http.StatusBadGateway)
		return
	}
	defer remote.Close()
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	// The HTTP parser may already have read tunnel bytes from the client.
	stop := context.AfterFunc(r.Context(), func() { _ = client.Close(); _ = remote.Close() })
	defer stop()
	relay(client, remote, buffered)

}

var proxyTransport = func() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = true
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	return transport
}()

func stripHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			header.Del(strings.TrimSpace(name))
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Proxy-Authorization", "Proxy-Authenticate", "Keep-Alive", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}

func handleHttp(w http.ResponseWriter, req *http.Request) {
	if req.URL.Host == "" || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
		http.Error(w, "absolute HTTP URL required", http.StatusBadRequest)
		return
	}
	out := req.Clone(req.Context())
	out.RequestURI = ""
	stripHopHeaders(out.Header)
	resp, err := proxyTransport.RoundTrip(out)
	if err != nil {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	stripHopHeaders(resp.Header)
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	// Flush each upstream chunk so small SSE events are not held in buffers.
	buffer := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := w.Write(buffer[:n]); writeErr != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}
