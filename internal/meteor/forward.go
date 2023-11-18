package meteor

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/dushxiiang/meteor/internal/location"
	"github.com/dushxiiang/meteor/pkg/logger"
)

var (
	UDPPacketSize = 8 * 1024
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
	sugar := logger.L.Sugar()
	ln, err := net.Listen("tcp", preprocessingAddr(r.Addr))
	if err != nil {
		sugar.Error("error listening address", err)
		return
	}
	defer ln.Close()
	sugar.Infof("TCP forwarder started: %s -> %s", ln.Addr().String(), r.To)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			sugar.Warn("error accept remoteConn", err)
			continue
		}
		tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
		if ok {
			if !r.Rules.Allowed(tcpAddr.IP, ipLocation) {
				_ = conn.Close()
				continue
			}
		}

		sugar.Debugf("TCP client connected, %s <- %s", conn.LocalAddr(), conn.RemoteAddr())
		go func() {
			defer conn.Close()
			backend, err := net.DialTimeout("tcp", r.To, time.Duration(Timeout)*time.Second)
			if err != nil {
				sugar.Errorf("forward to %s err: %v", r.To, err)
				return
			}
			sugar.Debugf("Meteor TCP client connected, %s -> %s", backend.LocalAddr(), backend.RemoteAddr())
			sugar.Debugf("Start mutual copy...")
			mutualCopyIO(backend, conn)
			sugar.Debugf("TCP client disconnected, %s <- %s", conn.LocalAddr(), conn.RemoteAddr())
			sugar.Debugf("Meteor TCP client disconnected, %s -> %s", backend.LocalAddr(), backend.RemoteAddr())
		}()
	}
}

func NewUDPForwarder() *UDPForwarder {
	return &UDPForwarder{
		udpConnMap: make(map[string]*UDPConnWrap),
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
	sugar := logger.L.Sugar()
	src, err := net.ResolveUDPAddr("udp", preprocessingAddr(r.Addr))
	if err != nil {
		sugar.Error("error resolving local address", err)
		return
	}
	dst, err := net.ResolveUDPAddr("udp", preprocessingAddr(r.To))
	if err != nil {
		sugar.Error("error resolving remote address", err)
		return
	}

	localConn, err := net.ListenUDP("udp", src)
	if err != nil {
		sugar.Error("error listening on local address", err)
		return
	}
	defer localConn.Close()

	sugar.Infof("UDP forwarder started: %s -> %s", src, dst)

	udpForwarder := NewUDPForwarder()
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
			sugar.Warn("Error reading from UDP:", err)
			return
		}

		if !r.Rules.Allowed(clientAddr.IP, ipLocation) {
			continue
		}

		udpConnWrap, ok := udpForwarder.Get(clientAddr.String())
		if !ok {
			sugar.Debugf("UDP client connected, %s <- %s", localConn.LocalAddr(), clientAddr)
			// 创建远程UDP连接
			remoteConn, err := net.DialUDP("udp", nil, dst)
			if err != nil {
				sugar.Warn("Error connecting to remote address:", err)
				continue
			}
			sugar.Debugf("Meteor UDP client connected, %s -> %s", remoteConn.LocalAddr(), remoteConn.RemoteAddr())
			udpConnWrap = &UDPConnWrap{
				clientAddr: clientAddr,
				localConn:  localConn,
				remoteConn: remoteConn,
			}
			udpForwarder.Set(clientAddr.String(), udpConnWrap)

			go func() {
				defer udpForwarder.Del(clientAddr.String())
				udpConnWrap.Loop()
			}()
		}

		// 发送数据到远程地址
		_, err = udpConnWrap.Write(buffer[:n])
		if err != nil {
			sugar.Warn("Error forwarding data:", err)
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
	sugar := logger.L.Sugar()
	defer func() {
		sugar.Debugf("UDP client disconnected, %s <- %s", r.localConn.LocalAddr(), r.clientAddr)
		sugar.Debugf("Meteor UDP client disconnected, %s -> %s", r.remoteConn.LocalAddr(), r.remoteConn.RemoteAddr())
	}()

	buffer := make([]byte, UDPPacketSize)
	for {
		n, err := r.Read(buffer)
		if err != nil {
			sugar.Warn("Error reading from remote UDP:", err)
			return
		}

		_, err = r.localConn.WriteToUDP(buffer[:n], r.clientAddr)
		if err != nil {
			sugar.Warn("Error forwarding data to local:", err)
			return
		}
	}

}
