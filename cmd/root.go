package cmd

import (
	"fmt"
	"os"

	"github.com/dushxiiang/meteor/internal/meteor"
	"github.com/dushxiiang/meteor/pkg/logger"

	"github.com/spf13/cobra"
)

var (
	config string
	debug  bool
)

var rootCmd = &cobra.Command{
	Use:   "meteor",
	Short: "Meteor is a network tool that can quickly forward tcp and udp ports and start http, https and socks5 proxy servers.",
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		if debug {
			logger.L.Sugar().Debug("Current processing debug mode")
		}
		m, err := meteor.New(config)
		if err != nil {
			logger.L.Sugar().Fatal(err)
		}
		if err := m.InitLocationService(); err != nil {
			logger.L.Sugar().Warn(err)
		}
		m.Run()
	},
}

func Execute() {
	rootCmd.PersistentFlags().StringVarP(&config, "config", "c", "/etc/meteor/meteor.yaml", "-c /path/config.yaml")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "print debug log")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
