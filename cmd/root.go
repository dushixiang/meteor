package cmd

import (
	"log/slog"
	"os"

	"github.com/dushxiiang/meteor/internal/meteor"

	"github.com/spf13/cobra"
)

var (
	config string
	debug  bool
)

var rootCmd = &cobra.Command{
	Use:           "meteor",
	Short:         "Meteor is a network tool that can quickly forward tcp and udp ports and start http, https and socks5 proxy servers.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if debug {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if debug {
			slog.Debug("debug logging enabled")
		}
		m, err := meteor.New(config)
		if err != nil {
			return err
		}
		srv, err := getService(config, m)
		if err != nil {
			_ = m.Stop(nil)
			return err
		}
		return srv.Run()
	},
}

func Execute() {
	rootCmd.PersistentFlags().StringVarP(&config, "config", "c", "/etc/meteor/meteor.yaml", "-c /path/config.yaml")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "print debug log")
	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
