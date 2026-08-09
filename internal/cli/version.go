package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sbekti/intern-api/internal/buildinfo"
	"github.com/sbekti/intern-api/internal/cli/config"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show release version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), buildinfo.Details(config.DefaultServerURL))
			return err
		},
	}
}
