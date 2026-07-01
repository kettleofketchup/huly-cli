package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

var (
	authEndpoint  string
	authWorkspace string
	authToken     string
)

var authCmd = &cobra.Command{Use: "auth", Short: "Manage API authentication"}

var authSetTokenCmd = &cobra.Command{
	Use:   "set-token",
	Short: "Store a pre-generated Huly app/API token (non-interactive)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if authEndpoint == "" || authWorkspace == "" || authToken == "" {
			return fmt.Errorf("--endpoint, --workspace and --token are required")
		}
		return runSetToken(cmd.Context(), authEndpoint, authWorkspace, authToken)
	},
}

func runSetToken(ctx context.Context, endpoint, workspace, token string) error {
	rc := huly.NewRestClient(endpoint, workspace, token)
	account := ""
	if acct, err := rc.GetAccount(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not verify token via account lookup: %v\n", mapAuthErr(err))
	} else {
		account = acct.UUID
	}
	return creds.Save(creds.Credentials{Endpoint: endpoint, Workspace: workspace, Token: token, Account: account})
}

func init() {
	authSetTokenCmd.Flags().StringVar(&authEndpoint, "endpoint", "", "transactor REST base URL")
	authSetTokenCmd.Flags().StringVar(&authWorkspace, "workspace", "", "workspace uuid/url")
	authSetTokenCmd.Flags().StringVar(&authToken, "token", "", "app/API token")
	authCmd.AddCommand(authSetTokenCmd)
	rootCmd.AddCommand(authCmd)
}
