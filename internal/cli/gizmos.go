package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/sbekti/intern/internal/api"
	"github.com/sbekti/intern/internal/httpclient"
	"github.com/sbekti/intern/internal/session"
)

func newGizmoCommand(options *RootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "gizmo", Short: "Manage Gizmos"}
	cmd.AddCommand(newGizmosListCommand(options))
	cmd.AddCommand(newGizmosCreateCommand(options))
	cmd.AddCommand(newGizmosUpdateCommand(options))
	cmd.AddCommand(newGizmosDeleteCommand(options))
	return cmd
}

func newGizmosListCommand(options *RootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Gizmos",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutputFormat(output); err != nil {
				return err
			}
			client, err := authenticatedClient(options)
			if err != nil {
				return err
			}
			items, err := client.ListGizmos(cmd.Context())
			if err != nil {
				return mapGizmoMutationError(err, "list")
			}
			if output == "json" {
				return printJSON(cmd, items)
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				status := "unconfigured"
				if item.KioskUrl != nil {
					status = "configured"
				}
				rows = append(rows, []string{item.NetworkDevice.Id.String(), item.NetworkDevice.DisplayName, item.NetworkDevice.MacAddress, status})
			}
			return printTable(cmd, []string{"DEVICE ID", "NAME", "MAC ADDRESS", "KIOSK"}, rows)
		},
	}
	addOutputFlag(cmd, &output)
	return cmd
}

func newGizmosCreateCommand(options *RootOptions) *cobra.Command {
	var deviceID string
	var kioskURL string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Register a network device as a Gizmo",
		RunE: func(cmd *cobra.Command, _ []string) error {
			parsedID, err := uuid.Parse(deviceID)
			if err != nil {
				return fmt.Errorf("invalid --device-id: %w", err)
			}
			body := api.GizmoWrite{NetworkDeviceId: parsedID}
			if cmd.Flags().Changed("kiosk-url") {
				body.KioskUrl = &kioskURL
			}
			client, err := authenticatedClient(options)
			if err != nil {
				return err
			}
			created, err := client.CreateGizmo(cmd.Context(), body)
			if err != nil {
				return mapGizmoMutationError(err, "create")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created Gizmo %s (%s).\n", created.NetworkDevice.DisplayName, created.NetworkDevice.Id.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "Network device ID")
	cmd.Flags().StringVar(&kioskURL, "kiosk-url", "", "Kiosk destination URL")
	_ = cmd.MarkFlagRequired("device-id")
	return cmd
}

func newGizmosUpdateCommand(options *RootOptions) *cobra.Command {
	var kioskURL string
	cmd := &cobra.Command{
		Use:   "update <device-id>",
		Short: "Update a Gizmo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("kiosk-url") {
				return errors.New("--kiosk-url is required")
			}
			client, err := authenticatedClient(options)
			if err != nil {
				return err
			}
			updated, err := client.UpdateGizmo(cmd.Context(), args[0], api.GizmoUpdate{KioskUrl: &kioskURL})
			if err != nil {
				return mapGizmoMutationError(err, "update")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated Gizmo %s (%s).\n", updated.NetworkDevice.DisplayName, updated.NetworkDevice.Id.String())
			return nil
		},
	}
	cmd.Flags().StringVar(&kioskURL, "kiosk-url", "", "Kiosk destination URL (empty clears it)")
	return cmd
}

func newGizmosDeleteCommand(options *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <device-id>",
		Short: "Remove a Gizmo without deleting its network device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := authenticatedClient(options)
			if err != nil {
				return err
			}
			if err := client.DeleteGizmo(cmd.Context(), args[0]); err != nil {
				return mapGizmoMutationError(err, "delete")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted Gizmo %s.\n", args[0])
			return nil
		},
	}
}

func authenticatedClient(options *RootOptions) (*httpclient.Client, error) {
	runtime, err := resolveRuntime(options)
	if err != nil {
		return nil, err
	}
	return runtime.NewAuthenticatedClient()
}

func mapGizmoMutationError(err error, action string) error {
	if errors.Is(err, session.ErrSessionNotFound) {
		return errors.New("not logged in")
	}
	if errors.Is(err, httpclient.ErrUnauthorized) {
		return errors.New("authentication failed; run `internctl login` again")
	}
	if errors.Is(err, httpclient.ErrForbidden) {
		return fmt.Errorf("admin access is required to %s Gizmos", action)
	}
	var apiErr httpclient.APIError
	if errors.As(err, &apiErr) && strings.TrimSpace(apiErr.Message) != "" {
		return errors.New(apiErr.Message)
	}
	return err
}
