package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/output"
)

var projectCmd = &cobra.Command{Use: "project", Short: "Manage projects"}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, _, err := newClient()
		if err != nil {
			return err
		}
		ps, err := listProjects(cmd.Context(), rc)
		if err != nil {
			return err
		}
		if viper.GetString("output") == "json" {
			return output.JSON(os.Stdout, ps)
		}
		rows := make([][]string, 0, len(ps))
		for _, p := range ps {
			rows = append(rows, []string{p.Identifier, p.Name, p.ID})
		}
		output.Table(os.Stdout, []string{"KEY", "NAME", "ID"}, rows)
		return nil
	},
}

func listProjects(ctx context.Context, rc *huly.RestClient) ([]huly.Project, error) {
	var ps []huly.Project
	_, err := rc.FindAll(ctx, huly.ClassProject, nil, nil, &ps)
	if err != nil {
		return nil, mapAuthErr(err)
	}
	return ps, nil
}

func init() {
	projectListCmd.ValidArgsFunction = cobra.NoFileCompletions
	projectCmd.AddCommand(projectListCmd)
	rootCmd.AddCommand(projectCmd)
}
