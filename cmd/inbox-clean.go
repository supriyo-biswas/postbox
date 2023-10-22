package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	ent "github.com/supriyo-biswas/postbox/entities"
	"gorm.io/gorm"
)

func runInboxCleanCmd(cmd *cobra.Command, args []string) error {
	cfg, err := readConfig(cmd.Root().PersistentFlags())
	if err != nil {
		return fmt.Errorf("failed to read config: %s", err)
	}

	d, err := openDb(cfg.Database.Path)
	if err != nil {
		return err
	}

	var inbox ent.Inbox
	err = d.Where("name = ?", args[0]).First(&inbox).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("inbox %s not found", args[0])
		}
		return fmt.Errorf("failed to query inbox: %s", err)
	}

	var email ent.Email
	err = d.Where("inbox_id = ?", inbox.Id).Delete(&email).Error
	if err != nil {
		return fmt.Errorf("failed to delete emails: %s", err)
	}

	return nil
}

var inboxCleanCmd = &cobra.Command{
	Use:          "clean inbox",
	Aliases:      []string{"clear"},
	Short:        "Delete all emails in an inbox",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runInboxCleanCmd,
}
