package cli

import (
	"github.com/spf13/cobra"

	"github.com/sbekti/intern-api/internal/buildinfo"
	"github.com/sbekti/intern-api/internal/cli/config"
)

type RootOptions struct {
	Profile   string
	ConfigDir string
}

func NewRootCommand() *cobra.Command {
	options := &RootOptions{}

	cmd := &cobra.Command{
		Use:           "internctl",
		Short:         "Command-line client for the internal management platform",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Short(),
	}

	cmd.PersistentFlags().StringVar(
		&options.Profile,
		"profile",
		config.DefaultProfile,
		"Profile name to use",
	)
	cmd.PersistentFlags().StringVar(
		&options.ConfigDir,
		"config-dir",
		"",
		"Override the config directory (defaults to ~/.intern)",
	)

	cmd.AddCommand(newLoginCommand(options))
	cmd.AddCommand(newLogoutCommand(options))
	cmd.AddCommand(newWhoamiCommand(options))
	cmd.AddCommand(newSessionCommand(options))
	cmd.AddCommand(newVlanCommand(options))
	cmd.AddCommand(newDeviceCommand(options))
	cmd.AddCommand(newVersionCommand())

	return cmd
}
