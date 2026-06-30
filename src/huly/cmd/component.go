package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/cache"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/output"
)

var (
	compProject string
	compDesc    string
	compLead    string
)

var componentCmd = &cobra.Command{Use: "component", Short: "Manage components"}

var componentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List components",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, _, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(compProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		cs, err := listComponents(cmd.Context(), rc, pr.ID)
		if err != nil {
			return err
		}
		if viper.GetString("output") == "json" {
			return output.JSON(os.Stdout, cs)
		}
		rows := make([][]string, 0, len(cs))
		for _, c := range cs {
			rows = append(rows, []string{c.Label, c.Description, c.ID})
		}
		output.Table(os.Stdout, []string{"LABEL", "DESCRIPTION", "ID"}, rows)
		return nil
	},
}

var componentGetCmd = &cobra.Command{
	Use:               "get <label|id>",
	Short:             "Show one component",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeComponents,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, _, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(compProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		cs, err := listComponents(cmd.Context(), rc, pr.ID)
		if err != nil {
			return err
		}
		for _, c := range cs {
			if c.Label == args[0] || c.ID == args[0] {
				if viper.GetString("output") == "json" {
					return output.JSON(os.Stdout, c)
				}
				output.Table(os.Stdout,
					[]string{"LABEL", "DESCRIPTION", "LEAD", "ID"},
					[][]string{{c.Label, c.Description, c.Lead, c.ID}})
				return nil
			}
		}
		return fmt.Errorf("component %q not found", args[0])
	},
}

var componentCreateCmd = &cobra.Command{
	Use:     "create",
	Aliases: []string{"add"},
	Short:   "Create a component in Huly (and update the local cache)",
	RunE: func(cmd *cobra.Command, args []string) error {
		label, _ := cmd.Flags().GetString("label")
		if label == "" {
			return fmt.Errorf("--label is required")
		}
		rc, cr, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(compProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		id, err := createComponent(cmd.Context(), rc, pr.ID, pr.Identifier, cr.Account, label, compDesc, compLead)
		if err != nil {
			return err
		}
		fmt.Printf("created component %s (%s)\n", label, id)
		return nil
	},
}

// listComponents fetches all components belonging to a project space.
func listComponents(ctx context.Context, rc *huly.RestClient, projectRef string) ([]huly.Component, error) {
	var cs []huly.Component
	_, err := rc.FindAll(ctx, huly.ClassComponent, map[string]any{"space": projectRef}, nil, &cs)
	if err != nil {
		return nil, mapAuthErr(err)
	}
	return cs, nil
}

// createComponent posts a TxCreateDoc for a Component and writes through to the local cache.
func createComponent(ctx context.Context, rc *huly.RestClient, projectRef, projectIdent, account, label, desc, lead string) (string, error) {
	attrs := map[string]any{"label": label}
	if desc != "" {
		attrs["description"] = desc
	}
	if lead != "" {
		attrs["lead"] = lead
	}
	tx := huly.NewCreateDocTx(huly.ClassComponent, projectRef, attrs, account, nowMillis())
	if err := rc.Tx(ctx, tx); err != nil {
		return "", mapAuthErr(err)
	}
	id := tx["objectId"].(string)
	_ = cache.Update(func(c *cache.Cache) {
		c.Components = append(c.Components, cache.ComponentRec{ID: id, Project: projectIdent, Label: label})
	})
	return id, nil
}

func init() {
	componentCmd.PersistentFlags().StringVar(&compProject, "project", "", "project identifier")
	componentCreateCmd.Flags().String("label", "", "component label")
	componentCreateCmd.Flags().StringVar(&compDesc, "description", "", "description")
	componentCreateCmd.Flags().StringVar(&compLead, "lead", "", "lead (person ref)")
	// Register completion on the parent's persistent --project flag once. Do NOT
	// register on componentCreateCmd before AddCommand — the flag isn't visible
	// to it yet and the call would silently no-op.
	_ = componentCmd.RegisterFlagCompletionFunc("project", completeProjects)
	componentCmd.AddCommand(componentListCmd, componentGetCmd, componentCreateCmd)
	rootCmd.AddCommand(componentCmd)
}
