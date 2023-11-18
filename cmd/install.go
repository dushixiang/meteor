package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dushxiiang/meteor/internal/meteor"
	"github.com/dushxiiang/meteor/pkg/logger"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var defaultConfig = `
//geoip:
//  file: /etc/meteor/GeoLite2-City.mmdb
forwarders:
  - protocol: tcp
    addr: ":8080"
    to: 127.0.0.1:80
    rules:
      - city: beijing,chengdu
        allowed: true
      - ip: 0.0.0.0/0
        allowed: false
#  - protocol: udp
#    addr: ":54321"
#    to: 127.0.0.1:12345
#proxies:
#  - protocol: http
#    addr: 127.0.0.1:8080
#    auth: true
#    accounts:
#      - username: a
#        password: b
#  - protocol: https
#    addr: 127.0.0.1:80
#    key: /root/key.pem
#    cert: /root/cert.pem
#  - protocol: socks5
#    addr: 127.0.0.1:1080
`

func createConfigIfNotExists(filePath, exampleConfig string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		if err := os.WriteFile(filePath, []byte(exampleConfig), 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install meteor as a system service",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		err := createConfigIfNotExists(config, defaultConfig)
		if err != nil {
			logger.L.Sugar().Fatal("create config file", err)
		}
		control(config, cmd.Use)
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall meteor system service",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		control(config, cmd.Use)
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start meteor system service",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		control(config, cmd.Use)
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop meteor system service",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		control(config, cmd.Use)
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart meteor system service",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		control(config, cmd.Use)
	},
}

func control(config, action string) {
	srv, err := getService(config)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	if err := service.Control(srv, action); err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	fmt.Printf("success\n")
}

func getService(config string) (service.Service, error) {
	svcConfig := &service.Config{
		Name:        "meteor",
		DisplayName: "meteor service",
		Description: "This is meteor service.",
		Arguments:   []string{"-c", config},
		Dependencies: []string{
			"Requires=network.target",
			"After=network-online.target syslog.target",
		},
		Option: service.KeyValue{
			"Restart": "always",
		},
	}

	if debug {
		svcConfig.Arguments = append(svcConfig.Arguments, "-d")
	}

	prg, err := meteor.New(config)
	if err != nil {
		return nil, errors.New("error to new meteor, " + err.Error())
	}
	return service.New(prg, svcConfig)
}
