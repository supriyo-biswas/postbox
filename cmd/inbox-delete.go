package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	ent "github.com/supriyo-biswas/postbox/entities"
	"gorm.io/gorm"
)

func runInboxRemoveCmd(cmd *cobra.Command, args []string) error {
	cfg, err := readConfig(cmd.Root().PersistentFlags())
	if err != nil {
		return fmt.Errorf("failed to read config: %s", err)
	}

	d, err := openDb(cfg.Database.Path)
	if err != nil {
		return err
	}

	var inbox ent.Inbox
	err = d.Select("id").Where("name = ?", args[0]).First(&inbox).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("inbox %s not found", args[0])
		}
		return fmt.Errorf("failed to query inbox: %s", err)
	}

	if err := d.Delete(&inbox).Error; err != nil {
		return fmt.Errorf("failed to delete inbox: %s", err)
	}

	return nil
}

var inboxRemoveCmd = &cobra.Command{
	Use:          "remove inbox",
	Aliases:      []string{"rm", "delete"},
	Short:        "Delete an inbox",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runInboxRemoveCmd,
}
