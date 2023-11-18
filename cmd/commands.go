package cmd

import (
	"context"

	"github.com/dushxiiang/meteor/meteor"
	"github.com/dushxiiang/meteor/pkg/logger"

	"github.com/spf13/cobra"
)

var forwardModel meteor.Forwarder
var forwardCmd = &cobra.Command{
	Use:        "forward",
	SuggestFor: []string{"f"},
	Short:      "Forward the received data to the destination address",
	Args:       cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		ctx := context.Background()
		forwardModel.Addr = args[0]
		forwardModel.To = args[1]
		forwardModel.Forward(ctx)
	},
}

var proxyModel meteor.Proxy
var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Start a proxy server",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger.Init(debug)
		ctx := context.Background()
		proxyModel.Addr = args[0]
		if len(args) == 3 {
			proxyModel.Cert = args[1]
			proxyModel.Key = args[2]
		}
		proxyModel.Run(ctx)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		println(meteor.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	forwardCmd.PersistentFlags().StringVarP(&forwardModel.Protocol, "protocol", "p", "tcp", "tcp|udp")
	forwardCmd.PersistentFlags().StringVarP(&forwardModel.Addr, "addr", "a", "", "8080|0.0.0.0:880")
	forwardCmd.PersistentFlags().StringVarP(&forwardModel.To, "to", "t", "", "127.0.0.1:80")
	rootCmd.AddCommand(forwardCmd)

	proxyCmd.PersistentFlags().StringVarP(&proxyModel.Protocol, "protocol", "p", "http", "protocol: http|https|socks5")
	proxyCmd.PersistentFlags().StringVarP(&proxyModel.Addr, "addr", "a", "", "8080|0.0.0.0:880")
	proxyCmd.PersistentFlags().StringVar(&proxyModel.Cert, "cert", "", "only for https, /path/cert.pem")
	proxyCmd.PersistentFlags().StringVar(&proxyModel.Key, "key", "", "only for https, /path/key.pem")

	rootCmd.AddCommand(proxyCmd)

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
}
