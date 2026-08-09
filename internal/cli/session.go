package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sbekti/intern-api/internal/api"
	"github.com/sbekti/intern-api/internal/httpclient"
	"github.com/sbekti/intern-api/internal/session"
)

func newSessionCommand(options *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage API client sessions",
	}
	cmd.AddCommand(newSessionListCommand(options))
	cmd.AddCommand(newSessionRevokeCommand(options))
	cmd.AddCommand(newSessionRevokeAllCommand(options))
	return cmd
}

func newSessionListCommand(options *RootOptions) *cobra.Command {
	var output string
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active client sessions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOutputFormat(output); err != nil {
				return err
			}
			runtime, err := resolveRuntime(options)
			if err != nil {
				return err
			}

			client, err := runtime.NewAuthenticatedClient()
			if err != nil {
				return err
			}

			sessions, err := listAllSessions(cmd.Context(), client, all)
			if err != nil {
				return mapSessionError(err, all, "list")
			}

			if output == "json" {
				return printJSON(cmd, sessions)
			}

			return printSessionTable(cmd, sessions, all)
		},
	}

	addOutputFlag(cmd, &output)
	cmd.Flags().BoolVar(&all, "all", false, "List client sessions across all users (admin only)")

	return cmd
}

func newSessionRevokeCommand(options *RootOptions) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a client session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runtime, err := resolveRuntime(options)
			if err != nil {
				return err
			}

			client, err := runtime.NewAuthenticatedClient()
			if err != nil {
				return err
			}

			if err := client.RevokeSession(cmd.Context(), args[0], all); err != nil {
				return mapSessionError(err, all, "revoke")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Revoked session %s.\n", args[0])
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Revoke a client session across all users (admin only)")
	return cmd
}

func newSessionRevokeAllCommand(options *RootOptions) *cobra.Command {
	var all bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "revoke-all",
		Short: "Revoke multiple client sessions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				message := "Revoke all of your other client sessions?"
				if all {
					message = "Revoke all client sessions across all users? This may sign out your current CLI session."
				}
				ok, err := confirmAction(cmd, message)
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("aborted")
				}
			}

			runtime, err := resolveRuntime(options)
			if err != nil {
				return err
			}

			client, err := runtime.NewAuthenticatedClient()
			if err != nil {
				return err
			}

			if err := client.RevokeAllSessions(cmd.Context(), all); err != nil {
				return mapSessionError(err, all, "revoke")
			}

			if all {
				fmt.Fprintln(cmd.OutOrStdout(), "Revoked all client sessions across all users.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Revoked all other client sessions.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Revoke client sessions across all users (admin only)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

const sessionPageSize int32 = 200

func listAllSessions(ctx context.Context, client *httpclient.Client, all bool) ([]api.AuthSession, error) {
	sessions := make([]api.AuthSession, 0)
	offset := int32(0)
	for pageNumber := 0; pageNumber < 10000; pageNumber++ {
		page, err := client.ListSessions(ctx, all, sessionPageSize, offset)
		if err != nil {
			return nil, err
		}
		if page == nil {
			return nil, errors.New("list sessions: empty response")
		}
		if page.Pagination.Total < 0 || page.Pagination.Offset < 0 || page.Pagination.Offset != offset || page.Pagination.Limit < 1 || page.Pagination.Limit > sessionPageSize {
			return nil, errors.New("list sessions: malformed pagination")
		}
		if len(page.Items) > int(page.Pagination.Limit) {
			return nil, errors.New("list sessions: malformed pagination")
		}
		sessions = append(sessions, page.Items...)
		if page.Pagination.Total < int64(len(sessions)) {
			return nil, errors.New("list sessions: malformed pagination")
		}
		if len(page.Items) == 0 {
			if offset == 0 && len(sessions) == 0 && page.Pagination.Total == 0 {
				return sessions, nil
			}
			return nil, errors.New("list sessions: malformed pagination")
		}
		if int64(len(sessions)) == page.Pagination.Total {
			return sessions, nil
		}
		nextOffset := offset + int32(len(page.Items))
		if nextOffset <= offset || nextOffset < page.Pagination.Offset {
			return nil, errors.New("list sessions: non-progressing pagination")
		}
		offset = nextOffset
	}
	return nil, errors.New("list sessions: pagination exceeded safety limit")
}

func printSessionTable(cmd *cobra.Command, sessions []api.AuthSession, all bool) error {
	rows := make([][]string, 0, len(sessions))
	for _, item := range sessions {
		row := []string{
			item.Id.String(),
		}
		if all {
			row = append(row, item.Username)
		}
		row = append(row,
			currentLabel(item.IsCurrent),
			item.ClientName,
			formatTime(item.LastUsedAt),
			item.IdleExpiresAt.Format(time.RFC3339),
			item.ExpiresAt.Format(time.RFC3339),
		)
		rows = append(rows, row)
	}

	headers := []string{"ID"}
	if all {
		headers = append(headers, "USERNAME")
	}
	headers = append(headers, "CURRENT", "CLIENT", "LAST USED", "IDLE EXPIRY", "ABSOLUTE EXPIRY")
	return printTable(cmd, headers, rows)
}

func mapSessionError(err error, all bool, action string) error {
	if errors.Is(err, session.ErrSessionNotFound) {
		return errors.New("not logged in")
	}
	if errors.Is(err, httpclient.ErrUnauthorized) {
		return errors.New("authentication failed; run `internctl login` again")
	}
	if errors.Is(err, httpclient.ErrForbidden) {
		if all {
			return fmt.Errorf("admin access is required to %s all-user sessions", action)
		}
		return fmt.Errorf("admin access is required to %s these sessions", action)
	}
	return err
}

func currentLabel(current bool) string {
	if current {
		return "yes"
	}
	return "no"
}

func formatTime(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format(time.RFC3339)
}

func confirmAction(cmd *cobra.Command, message string) (bool, error) {
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N]: ", message); err != nil {
		return false, err
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}
