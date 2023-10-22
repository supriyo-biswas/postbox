package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	ent "github.com/supriyo-biswas/postbox/entities"
	"github.com/supriyo-biswas/postbox/utils"
	"gorm.io/gorm"
)

func runInboxRotateCmd(cmd *cobra.Command, args []string) error {
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

	path, _ := cmd.Flags().GetString("credential-file")
	newCreds, err := newCredentials(path)
	if err != nil {
		return err
	}

	inbox.SmtpPass = utils.HashSecret(newCreds.SmtpPass)
	inbox.ApiKey = utils.HashSecret(newCreds.ApiKey)

	if err = d.Save(&inbox).Error; err != nil {
		return fmt.Errorf("failed to update inbox: %s", err)
	}

	fmt.Printf("Inbox ID: %d\n", inbox.Id)
	fmt.Printf("SMTP username: %s\n", args[0])
	if path == "" {
		fmt.Printf("SMTP password: %s\n", newCreds.SmtpPass)
		fmt.Printf("API key: %s\n", newCreds.ApiKey)
	}

	return nil
}

var inboxRotateCmd = &cobra.Command{
	Use:          "rotate inbox",
	Short:        "Rotate an inbox's SMTP password and API key",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runInboxRotateCmd,
}

func init() {
	inboxRotateCmd.Flags().StringP("credential-file", "f", "", "Use credentials from file")
}
