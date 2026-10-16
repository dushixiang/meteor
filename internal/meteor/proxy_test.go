package meteor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxyAuthentication(t *testing.T) {
	p := Proxy{Auth: true, Accounts: []Account{{Username: "alice", Password: "secret"}}}
	for _, value := range []string{"", "B", "Basic", "Basic YWxpY2U=", "Bearer token", "Basic !!!"} {
		r := httptest.NewRequest("GET", "http://example.com", nil)
		r.Header.Set("Proxy-Authorization", value)
		if p.auth(r) {
			t.Fatalf("accepted %q", value)
		}
	}
	r := httptest.NewRequest("GET", "http://example.com", nil)
	r.SetBasicAuth("alice", "secret")
	r.Header.Set("Proxy-Authorization", r.Header.Get("Authorization"))
	if !p.auth(r) {
		t.Fatal("valid auth rejected")
	}
	w := httptest.NewRecorder()
	p.writeBasicUnauthorized(w)
	if w.Result().StatusCode != 407 || w.Result().Header.Get("Proxy-Authenticate") == "" {
		t.Fatal("missing auth challenge")
	}
}

func TestHTTPHeadersAndStreaming(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range []string{"Proxy-Authorization", "Connection", "X-Hop"} {
			if r.Header.Get(h) != "" {
				t.Errorf("leaked %s", h)
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release
	}))
	defer upstream.Close()
	// Ensure server.Close cannot wait for the handler before release is closed.
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	proxy := httptest.NewServer(http.HandlerFunc(handleHttp))
	defer proxy.Close()
	req, _ := http.NewRequest("POST", upstream.URL, nil)
	req.Header.Set("Proxy-Authorization", "Basic secret")
	req.Header.Set("Connection", "X-Hop")
	req.Header.Set("X-Hop", "private")
	proxyURL, _ := url.Parse(proxy.URL)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyURL(proxyURL)
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := (&http.Client{Transport: transport}).Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || line != "data: first\n" {
		t.Fatalf("stream buffered: %q %v", line, err)
	}
}

func TestCONNECTPreservesBufferedBytesAndCloses(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer upstream.Close()
	proxy := httptest.NewServer(http.HandlerFunc(handleTunneling))
	defer proxy.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(proxy.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	host := strings.TrimPrefix(upstream.URL, "http://")
	fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\nGET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host, host, host)
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: "CONNECT"})
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("connect: %v %v", response, err)
	}
	tunneled, err := http.ReadResponse(reader, &http.Request{Method: "GET"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(tunneled.Body)
	tunneled.Body.Close()
	if err != nil || string(body) != "ok" {
		t.Fatalf("buffer lost: %s %v", body, err)
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		t.Fatalf("tunnel not closed: %v", err)
	}
}
