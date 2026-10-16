package cmd

import (
	"github.com/dushxiiang/meteor/internal/meteor"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(meteor.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCmd)
	for _, action := range []string{"uninstall", "start", "stop", "restart"} {
		rootCmd.AddCommand(&cobra.Command{
			Use:   action,
			Short: action + " meteor system service",
			RunE:  func(cmd *cobra.Command, args []string) error { return control(config, cmd.Name()) },
		})
	}
}
