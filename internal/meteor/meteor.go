package meteor

import (
	"context"
	"github.com/dushxiiang/meteor/internal/location"
	"github.com/dushxiiang/meteor/pkg/logger"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/kardianos/service"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const Timeout int = 10
const Version = "v0.1.0"

type Config struct {
	Forwarders []Forwarder    `yaml:"forwarders"`
	Proxies    []Proxy        `yaml:"proxies"`
	Location   LocationConfig `yaml:"location"`
}

type LocationConfig struct {
	Type string `yaml:"type"`
	File string `yaml:"file"`
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
		quit:   make(chan struct{}),
	}
	return &meteor, nil
}

type Meteor struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *Config

	Location location.Location
	quit     chan struct{}
}

func (r *Meteor) Start(s service.Service) error {
	go r.Run()
	return nil
}

func (r *Meteor) Run() {
	forwarders := r.cfg.Forwarders
	for i := range forwarders {
		go forwarders[i].Forward(r.ctx, r.Location)
	}

	proxies := r.cfg.Proxies
	for i := range proxies {
		go proxies[i].Run(r.ctx)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	select {
	case <-interrupt:
		close(r.quit)
	case <-r.quit:
		r.cancel()
	}
}

func (r *Meteor) Stop(s service.Service) error {
	close(r.quit)
	return nil
}

func (r *Meteor) InitLocationService() error {
	locationConfig := r.cfg.Location
	switch locationConfig.Type {
	case "geoip":
		ipLocation, err := location.NewGeoIPLocation(locationConfig.File)
		if err != nil {
			return err
		}
		r.Location = ipLocation
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
