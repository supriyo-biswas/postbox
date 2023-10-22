package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	ent "github.com/supriyo-biswas/postbox/entities"
)

func runInboxListCmd(cmd *cobra.Command, args []string) error {
	cfg, err := readConfig(cmd.Root().PersistentFlags())
	if err != nil {
		return fmt.Errorf("failed to read config: %s", err)
	}

	d, err := openDb(cfg.Database.Path)
	if err != nil {
		return err
	}

	var inboxes []ent.Inbox
	if err := d.Select("name").Find(&inboxes).Error; err != nil {
		return fmt.Errorf("failed to query inboxes: %s", err)
	}

	for _, inbox := range inboxes {
		fmt.Println(inbox.Name)
	}

	return nil
}

var inboxListCmd = &cobra.Command{
	Use:          "list inbox",
	Aliases:      []string{"ls"},
	Short:        "List inboxes",
	SilenceUsage: true,
	RunE:         runInboxListCmd,
}
