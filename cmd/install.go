package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var defaultConfig = `
#location:
#  type: geoip
#  file: /etc/meteor/GeoLite2-City.mmdb
forwarders:
  - protocol: tcp
    addr: "127.0.0.1:8080"
    to: 127.0.0.1:80
    rules:
      - ip: 0.0.0.0/0
        allowed: true
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
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(exampleConfig)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install meteor as a system service",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := createConfigIfNotExists(config, defaultConfig)
		if err != nil {
			return fmt.Errorf("create config file: %w", err)
		}
		return control(config, cmd.Name())
	},
}

func control(config, action string) error {
	srv, err := getService(config, nil)
	if err != nil {
		return err
	}
	if err := service.Control(srv, action); err != nil {
		return fmt.Errorf("%s service: %w", action, err)
	}
	fmt.Println("success")
	return nil
}

func getService(config string, program service.Interface) (service.Service, error) {
	config, err := filepath.Abs(config)
	if err != nil {
		return nil, err
	}
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
			"Restart":   "always",
			"LogOutput": false,
		},
	}

	if debug {
		svcConfig.Arguments = append(svcConfig.Arguments, "-d")
	}

	// Service control must work even when the runtime configuration is invalid.
	return service.New(program, svcConfig)
}
