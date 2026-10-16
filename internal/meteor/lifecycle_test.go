package meteor

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func configFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "meteor.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStrictConfig(t *testing.T) {
	for name, content := range map[string]string{
		"unknown":            "forwarder: []",
		"protocol":           "proxies: [{protocol: invalid, addr: ':8080'}]",
		"invalid IP":         "forwarders: [{protocol: tcp, addr: ':8080', to: 'localhost:80', rules: [{ip: typo}]}]",
		"missing geoip":      "forwarders: [{protocol: tcp, addr: ':8080', to: 'localhost:80', rules: [{city: Beijing}]}]",
		"missing TLS":        "proxies: [{protocol: https, addr: ':443'}]",
		"missing accounts":   "proxies: [{protocol: socks5, addr: ':1080', auth: true}]",
		"multiple documents": "{}\n---\n{}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readConfig(configFile(t, content)); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	if _, err := readConfig(configFile(t, "proxies: [{protocol: socks5, addr: ':1080', auth: true, accounts: [{username: a, password: b}]}]")); err != nil {
		t.Fatal(err)
	}
}

func TestStopIsConcurrentAndIdempotent(t *testing.T) {
	m, err := New(configFile(t, "{}"))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{}, 4)
	for range 4 {
		go func() { _ = m.Stop(nil); done <- struct{}{} }()
	}
	for range 4 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("stop blocked")
		}
	}
}

func tcpPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	client, err := net.DialTCP("tcp", nil, ln.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatal(err)
	}
	server, err := ln.AcceptTCP()
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	for _, c := range []*net.TCPConn{client, server} {
		c.SetDeadline(time.Now().Add(3 * time.Second))
		t.Cleanup(func() { c.Close() })
	}
	return client, server
}

func TestRelayPreservesResponseAfterHalfClose(t *testing.T) {
	client, left := tcpPair(t)
	right, backend := tcpPair(t)
	done := make(chan struct{})
	go func() { relay(left, right, left); close(done) }()
	if _, err := client.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	client.CloseWrite()
	request, err := io.ReadAll(backend)
	if err != nil || string(request) != "request" {
		t.Fatalf("request: %q %v", request, err)
	}
	response := bytes.Repeat([]byte("response"), 65536)
	go func() { backend.Write(response); backend.CloseWrite() }()
	got, err := io.ReadAll(client)
	if err != nil || !bytes.Equal(got, response) {
		t.Fatalf("response truncated: %d/%d %v", len(got), len(response), err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("relay did not exit")
	}
}

func availableTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func dialReady(t *testing.T, addr string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.SetDeadline(time.Now().Add(2 * time.Second))
			t.Cleanup(func() { conn.Close() })
			return conn
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSOCKSAuthenticationAndShutdown(t *testing.T) {
	for _, auth := range []bool{false, true} {
		t.Run(map[bool]string{false: "anonymous", true: "authenticated"}[auth], func(t *testing.T) {
			p := Proxy{Protocol: "socks5", Addr: availableTCPAddr(t), Auth: auth, Accounts: []Account{{Username: "a", Password: "b"}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() { p.Run(ctx); close(done) }()
			conn := dialReady(t, p.Addr)
			conn.Write([]byte{5, 2, 0, 2})
			reply := make([]byte, 2)
			if _, err := io.ReadFull(conn, reply); err != nil {
				t.Fatal(err)
			}
			want := byte(0)
			if auth {
				want = 2
			}
			if !bytes.Equal(reply, []byte{5, want}) {
				t.Fatalf("method: %v", reply)
			}
			if auth {
				conn.Write([]byte{1, 1, 'a', 1, 'b'})
				if _, err := io.ReadFull(conn, reply); err != nil || !bytes.Equal(reply, []byte{1, 0}) {
					t.Fatalf("auth: %v %v", reply, err)
				}
			}
			cancel()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("SOCKS handshake blocked shutdown")
			}
		})
	}
}

func TestTCPForwarderCancellation(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	f := Forwarder{Protocol: "tcp", Addr: availableTCPAddr(t), To: backend.Addr().String()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { f.Forward(ctx, nil); close(done) }()
	client := dialReady(t, f.Addr)
	upstream, err := backend.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	client.Write([]byte("x"))
	buf := make([]byte, 1)
	upstream.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(upstream, buf); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("TCP forwarder blocked shutdown")
	}
}

func TestUDPForwarderLargeDatagramAndCancellation(t *testing.T) {
	upstream, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	go func() {
		buffer := make([]byte, 65535)
		for {
			n, addr, err := upstream.ReadFromUDP(buffer)
			if err != nil {
				return
			}
			upstream.WriteToUDP(buffer[:n], addr)
		}
	}()
	reservation, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.LocalAddr().(*net.UDPAddr)
	reservation.Close()
	f := Forwarder{Protocol: "udp", Addr: addr.String(), To: upstream.LocalAddr().String()}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { f.Forward(ctx, nil); close(done) }()
	client, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	payload := bytes.Repeat([]byte("packet"), 1500)
	buffer := make([]byte, 65535)
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := client.Write(payload); err != nil {
			t.Fatal(err)
		}
		client.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, err := client.Read(buffer)
		if err == nil {
			if !bytes.Equal(buffer[:n], payload) {
				t.Fatalf("datagram truncated: %d/%d", n, len(payload))
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("UDP forwarder blocked shutdown")
	}
	rebound, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("listener leaked: %v", err)
	}
	rebound.Close()
}
