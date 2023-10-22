package cmd

import "github.com/spf13/cobra"

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Manage inboxes",
}

func init() {
	inboxCmd.AddCommand(inboxAddCmd)
	inboxCmd.AddCommand(inboxListCmd)
	inboxCmd.AddCommand(inboxCleanCmd)
	inboxCmd.AddCommand(inboxRemoveCmd)
	inboxCmd.AddCommand(inboxRotateCmd)
}
