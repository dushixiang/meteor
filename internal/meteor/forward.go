package meteor

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/dushxiiang/meteor/internal/location"
)

var (
	UDPPacketSize = 65535
	UDPTimeout    = time.Second * 30
)

type Forwarder struct {
	Protocol string  `yaml:"protocol"`
	Addr     string  `yaml:"addr"`
	To       string  `yaml:"to"`
	Rules    RuleSet `yaml:"rules"`
}

func (r *Forwarder) Forward(ctx context.Context, ipLocation location.Location) {
	switch r.Protocol {
	case "tcp":
		r.forwardTCP(ctx, ipLocation)
	case "udp":
		r.forwardUDP(ctx, ipLocation)
	}
}

func (r *Forwarder) forwardTCP(ctx context.Context, ipLocation location.Location) {
	ln, err := net.Listen("tcp", preprocessingAddr(r.Addr))
	if err != nil {
		slog.Error("TCP listen failed", "addr", r.Addr, "error", err)
		return
	}
	defer ln.Close()
	stop := context.AfterFunc(ctx, func() { _ = ln.Close() })
	defer stop()
	var workers sync.WaitGroup
	defer workers.Wait()
	slog.Info("TCP forwarder started", "addr", ln.Addr(), "target", r.To)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("TCP accept failed", "addr", ln.Addr(), "error", err)
			}
			return
		}
		tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
		if ok {
			if !r.Rules.Allowed(tcpAddr.IP, ipLocation) {
				_ = conn.Close()
				continue
			}
		}

		slog.Debug("TCP client connected", "local", conn.LocalAddr(), "remote", conn.RemoteAddr())
		workers.Go(func() {
			stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
			defer stop()
			defer conn.Close()
			backend, err := (&net.Dialer{Timeout: time.Duration(Timeout) * time.Second}).DialContext(ctx, "tcp", r.To)
			if err != nil {
				slog.Error("TCP upstream connection failed", "target", r.To, "error", err)
				return
			}
			slog.Debug("TCP upstream connected", "local", backend.LocalAddr(), "remote", backend.RemoteAddr())
			slog.Debug("TCP relay started")
			relay(backend, conn, backend)
			slog.Debug("TCP client disconnected", "local", conn.LocalAddr(), "remote", conn.RemoteAddr())
			slog.Debug("TCP upstream disconnected", "local", backend.LocalAddr(), "remote", backend.RemoteAddr())
		})
	}
}

type UDPForwarder struct {
	udpConnMap  map[string]*UDPConnWrap
	udpConnLock sync.Mutex
}

func (r *UDPForwarder) Get(key string) (*UDPConnWrap, bool) {
	r.udpConnLock.Lock()
	defer r.udpConnLock.Unlock()
	wrap, ok := r.udpConnMap[key]
	return wrap, ok
}

func (r *UDPForwarder) Del(key string) {
	r.udpConnLock.Lock()
	defer r.udpConnLock.Unlock()
	wrap, ok := r.udpConnMap[key]
	if ok {
		_ = wrap.remoteConn.Close()
		delete(r.udpConnMap, key)
	}
}

func (r *UDPForwarder) Set(key string, conn *UDPConnWrap) {
	r.udpConnLock.Lock()
	defer r.udpConnLock.Unlock()
	r.udpConnMap[key] = conn
}

func (r *Forwarder) forwardUDP(ctx context.Context, ipLocation location.Location) {
	src, err := net.ResolveUDPAddr("udp", preprocessingAddr(r.Addr))
	if err != nil {
		slog.Error("UDP listen address resolution failed", "addr", r.Addr, "error", err)
		return
	}
	dst, err := net.ResolveUDPAddr("udp", preprocessingAddr(r.To))
	if err != nil {
		slog.Error("UDP target resolution failed", "target", r.To, "error", err)
		return
	}

	localConn, err := net.ListenUDP("udp", src)
	if err != nil {
		slog.Error("UDP listen failed", "addr", src, "error", err)
		return
	}
	defer localConn.Close()
	stop := context.AfterFunc(ctx, func() { _ = localConn.Close() })
	defer stop()

	slog.Info("UDP forwarder started", "addr", localConn.LocalAddr(), "target", dst)

	udpForwarder := &UDPForwarder{udpConnMap: make(map[string]*UDPConnWrap)}
	var workers sync.WaitGroup
	defer workers.Wait()
	defer udpForwarder.Close()
	buffer := make([]byte, UDPPacketSize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 读取数据
		n, clientAddr, err := localConn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("UDP read failed", "addr", localConn.LocalAddr(), "error", err)
			}
			return
		}

		if !r.Rules.Allowed(clientAddr.IP, ipLocation) {
			continue
		}

		udpConnWrap, ok := udpForwarder.Get(clientAddr.String())
		if !ok {
			slog.Debug("UDP client connected", "local", localConn.LocalAddr(), "remote", clientAddr)
			// 创建远程UDP连接
			remoteConn, err := net.DialUDP("udp", nil, dst)
			if err != nil {
				slog.Warn("UDP upstream connection failed", "target", dst, "error", err)
				continue
			}
			slog.Debug("UDP upstream connected", "local", remoteConn.LocalAddr(), "remote", remoteConn.RemoteAddr())
			udpConnWrap = &UDPConnWrap{
				clientAddr: clientAddr,
				localConn:  localConn,
				remoteConn: remoteConn,
			}
			udpForwarder.Set(clientAddr.String(), udpConnWrap)

			workers.Go(func() {
				defer udpForwarder.Del(clientAddr.String())
				udpConnWrap.Loop()
			})
		}

		// 发送数据到远程地址
		_, err = udpConnWrap.Write(buffer[:n])
		if err != nil {
			slog.Warn("UDP upstream write failed", "client", clientAddr, "target", dst, "error", err)
			continue
		}
	}
}

type UDPConnWrap struct {
	clientAddr *net.UDPAddr
	localConn  *net.UDPConn
	remoteConn *net.UDPConn
}

func (r *UDPConnWrap) Read(b []byte) (n int, err error) {
	_ = r.remoteConn.SetDeadline(time.Now().Add(UDPTimeout))
	return r.remoteConn.Read(b)
}

func (r *UDPConnWrap) Write(data []byte) (int, error) {
	_ = r.remoteConn.SetDeadline(time.Now().Add(UDPTimeout))
	return r.remoteConn.Write(data)
}

func (r *UDPConnWrap) Loop() {
	defer func() {
		slog.Debug("UDP client disconnected", "local", r.localConn.LocalAddr(), "remote", r.clientAddr)
		slog.Debug("UDP upstream disconnected", "local", r.remoteConn.LocalAddr(), "remote", r.remoteConn.RemoteAddr())
	}()

	buffer := make([]byte, UDPPacketSize)
	for {
		n, err := r.Read(buffer)
		if err != nil {
			slog.Warn("UDP upstream read failed", "target", r.remoteConn.RemoteAddr(), "error", err)
			return
		}

		_, err = r.localConn.WriteToUDP(buffer[:n], r.clientAddr)
		if err != nil {
			slog.Warn("UDP client write failed", "client", r.clientAddr, "error", err)
			return
		}
	}

}

func (r *UDPForwarder) Close() {
	r.udpConnLock.Lock()
	defer r.udpConnLock.Unlock()
	for key, wrap := range r.udpConnMap {
		_ = wrap.remoteConn.Close()
		delete(r.udpConnMap, key)
	}
}
