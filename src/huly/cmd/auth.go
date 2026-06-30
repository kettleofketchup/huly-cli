package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
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
		return runSetToken(authEndpoint, authWorkspace, authToken)
	},
}

func runSetToken(endpoint, workspace, token string) error {
	return creds.Save(creds.Credentials{Endpoint: endpoint, Workspace: workspace, Token: token})
}

func init() {
	authSetTokenCmd.Flags().StringVar(&authEndpoint, "endpoint", "", "transactor REST base URL")
	authSetTokenCmd.Flags().StringVar(&authWorkspace, "workspace", "", "workspace uuid/url")
	authSetTokenCmd.Flags().StringVar(&authToken, "token", "", "app/API token")
	authCmd.AddCommand(authSetTokenCmd)
	rootCmd.AddCommand(authCmd)
}
