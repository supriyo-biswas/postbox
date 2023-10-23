package cmd

import (
	"fmt"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
)

var version = "unknown"

var rootCmd = &cobra.Command{
	Use:     "postbox",
	Short:   "Email testing server",
	Version: version,
}

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(inboxCmd)

	cfgFile, err := xdg.ConfigFile("postbox/config.toml")
	if err != nil {
		cfgFile = "$XDG_CONFIG_HOME/postbox/config.toml"
	}

	rootCmd.PersistentFlags().StringP("config", "c", "", fmt.Sprintf("config file (default: %s)", cfgFile))
}
