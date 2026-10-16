package meteor

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/dushxiiang/meteor/internal/location"
	"github.com/kardianos/service"
	"go.yaml.in/yaml/v3"
)

const Timeout int = 10
const Version = "v0.2.0"

type Config struct {
	Forwarders []Forwarder    `yaml:"forwarders"`
	Proxies    []Proxy        `yaml:"proxies"`
	Location   LocationConfig `yaml:"location"`
}

type LocationConfig struct {
	Type string `yaml:"type"`
	File string `yaml:"file"`
}

func readConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	defer file.Close()
	var cfg Config
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("config must contain exactly one YAML document")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Location.Type != "" && c.Location.Type != "geoip" {
		return fmt.Errorf("unsupported location type %q", c.Location.Type)
	}
	if c.Location.Type == "geoip" && c.Location.File == "" {
		return fmt.Errorf("location.file is required")
	}
	for i := range c.Forwarders {
		f := &c.Forwarders[i]
		if f.Protocol != "tcp" && f.Protocol != "udp" {
			return fmt.Errorf("forwarder %d: unsupported protocol %q", i, f.Protocol)
		}
		if _, _, err := net.SplitHostPort(preprocessingAddr(f.Addr)); f.Addr == "" || err != nil {
			return fmt.Errorf("forwarder %d: invalid listen address %q", i, f.Addr)
		}
		if host, port, err := net.SplitHostPort(f.To); err != nil || host == "" || port == "" {
			return fmt.Errorf("forwarder %d: invalid target %q", i, f.To)
		}
		for j := range f.Rules {
			if err := f.Rules[j].Init(); err != nil {
				return fmt.Errorf("forwarder %d rule %d: %w", i, j, err)
			}
			if f.Rules[j].City != "" && c.Location.Type != "geoip" {
				return fmt.Errorf("forwarder %d: city rules require geoip location", i)
			}
		}
	}
	for i, p := range c.Proxies {
		if p.Protocol != "http" && p.Protocol != "https" && p.Protocol != "socks5" {
			return fmt.Errorf("proxy %d: unsupported protocol %q", i, p.Protocol)
		}
		if _, _, err := net.SplitHostPort(preprocessingAddr(p.Addr)); p.Addr == "" || err != nil {
			return fmt.Errorf("proxy %d: invalid listen address %q", i, p.Addr)
		}
		if p.Protocol == "https" && (p.Cert == "" || p.Key == "") {
			return fmt.Errorf("proxy %d: HTTPS requires cert and key", i)
		}
		if p.Auth && len(p.Accounts) == 0 {
			return fmt.Errorf("proxy %d: auth requires accounts", i)
		}
		for _, a := range p.Accounts {
			if a.Username == "" || a.Password == "" {
				return fmt.Errorf("proxy %d: empty credentials", i)
			}
		}
	}
	return nil
}

func New(config string) (*Meteor, error) {
	cfg, err := readConfig(config)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	meteor := Meteor{
		ctx:    ctx,
		cancel: cancel,
		cfg:    cfg,
	}
	if cfg.Location.Type == "geoip" {
		meteor.Location, err = location.NewGeoIPLocation(cfg.Location.File)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("initialize location: %w", err)
		}
	}

	return &meteor, nil
}

type Meteor struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *Config

	Location  location.Location
	workers   sync.WaitGroup
	done      chan struct{}
	startOnce sync.Once
}

func (r *Meteor) start() {
	r.startOnce.Do(func() {
		r.done = make(chan struct{})
		for i := range r.cfg.Forwarders {
			r.workers.Go(func() { r.cfg.Forwarders[i].Forward(r.ctx, r.Location) })
		}
		for i := range r.cfg.Proxies {
			r.workers.Go(func() { r.cfg.Proxies[i].Run(r.ctx) })
		}
		go func() {
			<-r.ctx.Done()
			r.workers.Wait()
			if closer, ok := r.Location.(io.Closer); ok {
				_ = closer.Close()
			}
			close(r.done)
		}()
	})
}

func (r *Meteor) Start(service.Service) error { r.start(); return nil }

func (r *Meteor) Stop(service.Service) error {
	r.cancel()
	r.start()
	<-r.done
	return nil
}

// relay waits for both directions and preserves TCP half-close semantics.
func relay(left, right net.Conn, leftReader io.Reader) {
	defer left.Close()
	defer right.Close()
	done := make(chan struct{})
	copyStream := func(dst, src net.Conn, reader io.Reader) {
		_, err := io.Copy(dst, reader)
		if tcp, ok := dst.(interface{ CloseWrite() error }); ok && err == nil {
			_ = tcp.CloseWrite()
		} else {
			_ = dst.Close()
			_ = src.Close()
		}
	}
	go func() { copyStream(right, left, leftReader); close(done) }()
	copyStream(left, right, right)
	<-done
}

func preprocessingAddr(addr string) string {
	if !strings.Contains(addr, ":") {
		addr = "0.0.0.0:" + addr
	}
	return addr
}
