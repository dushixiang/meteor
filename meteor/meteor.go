package meteor

import (
	"context"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/dushxiiang/meteor/pkg/logger"

	"github.com/kardianos/service"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const Timeout int = 10
const Version = "v0.1.0"

type Config struct {
	Forwarders []Forwarder `yaml:"forwarders"`
	Proxies    []Proxy     `yaml:"proxies"`
}

func readConfig(config string) (cfg *Config, err error) {
	viper.SetConfigFile(config)
	if err := viper.ReadInConfig(); err != nil {
		//logger.L.Sugar().Warnf("read config err: %s", err.Error())
		return &Config{}, nil
	} else {
		err := viper.Unmarshal(&cfg, func(decoderConfig *mapstructure.DecoderConfig) {
			decoderConfig.TagName = "yaml"
		})
		if err != nil {
			logger.L.Sugar().Errorf("unmarshal config err: %s", err.Error())
			return nil, err
		}
	}
	for i := range cfg.Forwarders {
		for j := range cfg.Forwarders[i].Rules {
			if err := cfg.Forwarders[i].Rules[j].Init(); err != nil {
				return nil, errors.Wrap(err, "failed parse forwarder rules")
			}
		}
	}
	return cfg, nil
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
	return &meteor, nil
}

type Meteor struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *Config
}

func (r *Meteor) Start(s service.Service) error {
	go r.Run()
	return nil
}

func (r *Meteor) Run() {
	for _, f := range r.cfg.Forwarders {
		go f.Forward(r.ctx)
	}

	for _, proxy := range r.cfg.Proxies {
		go proxy.Run(r.ctx)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	r.cancel()
}

func (r *Meteor) Stop(s service.Service) error {
	return syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (r *Meteor) InitGeoIP() error {
	geoipFile := viper.GetString("geoip.file")
	if geoipFile == "" {
		return nil
	}
	if err := InitGeoIP(geoipFile); err != nil {
		return errors.Wrap(err, "error init geoip")
	}
	return nil
}

type ConnCopier struct {
	User, Backend io.ReadWriter
}

func (c ConnCopier) CopyFromBackend(errc chan<- error) {
	_, err := io.Copy(c.User, c.Backend)
	errc <- err
}

func (c ConnCopier) CopyToBackend(errc chan<- error) {
	_, err := io.Copy(c.Backend, c.User)
	errc <- err
}

func mutualCopyIO(conn0, conn1 net.Conn) {
	var cc = ConnCopier{
		User:    conn0,
		Backend: conn1,
	}
	var errc = make(chan error, 1)
	go cc.CopyFromBackend(errc)
	go cc.CopyToBackend(errc)
	<-errc
}

func preprocessingAddr(addr string) string {
	if !strings.Contains(addr, ":") {
		addr = "0.0.0.0:" + addr
	}
	return addr
}
