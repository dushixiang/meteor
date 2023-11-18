package meteor

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dushxiiang/meteor/pkg/logger"

	"github.com/armon/go-socks5"
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
	Username string
	Password string
}

func (p Proxy) Run(ctx context.Context) {
	switch p.Protocol {
	case "http":
		p.startHttpProxyServer(ctx)
	case "https":
		p.startHttpsProxyServer(ctx)
	case "socks5":
		p.startSocks5ProxyServer(ctx)
	}
}

func (p Proxy) startHttpProxyServer(ctx context.Context) {
	sugar := logger.L.Sugar()
	server := &http.Server{
		Addr: p.Addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if p.Auth && !p.auth(r) {
				p.writeBasicUnauthorized(w)
				return
			}
			r.Header.Del("Proxy-Connection")
			r.Header.Del("Proxy-Authenticate")
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else {
				handleHttp(w, r)
			}
		}),
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	sugar.Infof("HTTP proxy server started: %s, with auth enabled: %v", p.Addr, p.Auth)

	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			sugar.Error("shutting down the proxy server", err)
		}
	}()

	<-ctx.Done()
	if err := server.Shutdown(ctx); err != nil {
		sugar.Error("error shutdown the proxy server", err)
	}
}

func (p Proxy) writeBasicUnauthorized(w http.ResponseWriter) {
	w.WriteHeader(http.StatusProxyAuthRequired)
	//w.Header().Set("Proxy-Connection", "close")
	w.Header().Set("Proxy-Authenticate", "Basic realm=Restricted")
	_, _ = fmt.Fprint(w, http.StatusText(http.StatusProxyAuthRequired))
}

func (p Proxy) auth(r *http.Request) bool {
	// 获取代理认证信息
	authHeader := r.Header.Get("Proxy-Authorization")
	if authHeader == "" {
		return false
	}

	// 解码认证信息
	auth := authHeader[len("Basic "):]
	decodedAuth, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return false
	}

	// 检查用户名和密码
	parts := strings.SplitN(string(decodedAuth), ":", 2)
	username := parts[0]
	password := parts[1]

	var passed = false
	for _, account := range p.Accounts {
		if account.Username == username && account.Password == password {
			passed = true
			break
		}
	}
	return passed
}

func (p Proxy) startHttpsProxyServer(ctx context.Context) {
	sugar := logger.L.Sugar()
	server := &http.Server{
		Addr: p.Addr,
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

	sugar.Infof("HTTPS proxy server started: %s, with auth enabled: %v", p.Addr, p.Auth)

	go func() {
		err := server.ListenAndServeTLS(p.Cert, p.Key)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			sugar.Error("shutting down the proxy server", err)
		}
	}()

	<-ctx.Done()
	if err := server.Shutdown(ctx); err != nil {
		sugar.Error("error shutdown the proxy server", err)
	}
}

func (p Proxy) Valid(username, password string) bool {
	if !p.Auth {
		return true
	}
	var passed = false
	for _, account := range p.Accounts {
		if account.Username == username && account.Password == password {
			passed = true
			break
		}
	}
	return passed
}

func (p Proxy) startSocks5ProxyServer(ctx context.Context) {
	sugar := logger.L.Sugar()

	conf := &socks5.Config{
		Credentials: p,
	}

	// never return err
	server, _ := socks5.New(conf)

	// 指定监听地址和端口
	addr := preprocessingAddr(p.Addr)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		sugar.Error("error listening on address", err)
		return
	}

	sugar.Infof("Socks5 proxy server started: %s, with auth enabled: %v", addr, p.Auth)
	go func() {
		if err := server.Serve(ln); err != nil {
			sugar.Warnf("Socks5 proxy server stoped: %s", err.Error())
			return
		}
	}()
	<-ctx.Done()
	_ = ln.Close()
}

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	remoteConn, err := net.DialTimeout("tcp", r.Host, time.Duration(Timeout)*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}
	centralConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}

	go mutualCopyIO(remoteConn, centralConn)
}

func handleHttp(w http.ResponseWriter, req *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
