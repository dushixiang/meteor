package meteor

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/dushxiiang/meteor/pkg/logger"
)

type Forwarder struct {
	Protocol string  `yaml:"protocol"`
	Addr     string  `yaml:"addr"`
	To       string  `yaml:"to"`
	Rules    RuleSet `yaml:"rules"`
}

func (r Forwarder) Forward(ctx context.Context) {
	switch r.Protocol {
	case "tcp":
		r.forwardTCP(ctx)
	case "udp":
		ForwardUDP(ctx, r.Addr, r.To)
	}
}

func (r Forwarder) forwardTCP(ctx context.Context) {
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
			conn, err := ln.Accept()
			if err != nil {
				sugar.Warn("error accept conn", err)
				continue
			}
			tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
			if ok {
				if !r.Rules.Allowed(tcpAddr.IP) {
					_ = conn.Close()
					continue
				}
			}

			sugar.Debugf("client connected, %s <- %s", conn.LocalAddr(), conn.RemoteAddr())
			go func() {
				defer conn.Close()
				backend, err := net.DialTimeout("tcp", r.To, time.Duration(Timeout)*time.Second)
				if err != nil {
					sugar.Errorf("forward to %s err: %v", r.To, err)
					return
				}
				mutualCopyIO(backend, conn)
				sugar.Debugf("client disconnected, %s <- %s", conn.LocalAddr(), conn.RemoteAddr())
			}()
		}
	}
}

func ForwardUDP(ctx context.Context, addr, to string) {
	sugar := logger.L.Sugar()
	src, err := net.ResolveUDPAddr("udp", preprocessingAddr(addr))
	if err != nil {
		sugar.Error("error resolving local address", err)
		return
	}
	dst, err := net.ResolveUDPAddr("udp", preprocessingAddr(to))
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
	buffer := make([]byte, 2048)
	for {
		// 读取数据
		n, clientAddr, err := localConn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			return
		}

		// 打印接收到的数据
		fmt.Printf("Received %d bytes from %s: %s\n", n, clientAddr, buffer[:n])

		// 发送数据到远程地址
		_, err = localConn.WriteToUDP(buffer[:n], dst)
		if err != nil {
			fmt.Println("Error forwarding data:", err)
			return
		}
	}
}
