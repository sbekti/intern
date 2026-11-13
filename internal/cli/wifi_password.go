package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type wifiPasswordFlags struct {
	private      bool
	readPassword bool
}

func (f *wifiPasswordFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.private, "private-wifi-password", false, "Use a private Wi-Fi password for this device")
	cmd.Flags().BoolVar(&f.readPassword, "wifi-password", false, "Read a new Wi-Fi password (hidden terminal prompt or standard input)")
}

func (f wifiPasswordFlags) read(cmd *cobra.Command) (*bool, *string, error) {
	var enabled *bool
	if cmd.Flags().Changed("private-wifi-password") {
		enabled = &f.private
	}
	if !f.readPassword {
		return enabled, nil, nil
	}
	var value []byte
	var err error
	input, isFile := cmd.InOrStdin().(*os.File)
	if isFile && term.IsTerminal(int(input.Fd())) {
		fmt.Fprint(cmd.ErrOrStderr(), "New Wi-Fi password: ")
		value, err = term.ReadPassword(int(input.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
	} else {
		// Allow one trailing line ending; spaces are part of the password.
		value, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), 66))
		value = []byte(strings.TrimSuffix(strings.TrimSuffix(string(value), "\n"), "\r"))
	}
	if err != nil {
		return nil, nil, errors.New("could not read Wi-Fi password")
	}
	if len(value) < 8 || len(value) > 63 {
		return nil, nil, errors.New("Wi-Fi password must contain 8 to 63 printable ASCII characters")
	}
	for _, char := range value {
		if char < 0x20 || char > 0x7e {
			return nil, nil, errors.New("Wi-Fi password must contain 8 to 63 printable ASCII characters")
		}
	}
	password := string(value)
	return enabled, &password, nil
}

func newDevicePasswordCommand(options *RootOptions) *cobra.Command {
	run := func(cmd *cobra.Command, args []string) error {
		runtime, err := resolveRuntime(options)
		if err != nil {
			return err
		}
		client, err := runtime.NewAuthenticatedClient()
		if err != nil {
			return err
		}
		var password string
		if len(args) == 0 {
			password, err = client.GenerateWiFiPassword(cmd.Context())
		} else {
			password, err = client.RevealWiFiPassword(cmd.Context(), args[0])
		}
		if err != nil {
			return mapDeviceMutationError(err, "manage Wi-Fi passwords for")
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), password)
		return err
	}
	cmd := &cobra.Command{Use: "password", Short: "Generate or reveal a Wi-Fi password"}
	cmd.AddCommand(
		&cobra.Command{Use: "generate", Short: "Generate an unsaved passphrase", Args: cobra.NoArgs, RunE: run},
		&cobra.Command{Use: "show <id>", Short: "Reveal a saved Wi-Fi password", Args: cobra.ExactArgs(1), RunE: run},
	)
	return cmd
}
