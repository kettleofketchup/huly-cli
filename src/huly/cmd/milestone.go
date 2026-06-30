package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/cache"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/output"
)

var (
	msProject    string
	msTargetDate string
)

var milestoneCmd = &cobra.Command{Use: "milestone", Short: "Manage milestones"}

var milestoneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List milestones",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, _, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(msProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		ms, err := listMilestones(cmd.Context(), rc, pr.ID)
		if err != nil {
			return err
		}
		if viper.GetString("output") == "json" {
			return output.JSON(os.Stdout, ms)
		}
		rows := make([][]string, 0, len(ms))
		for _, m := range ms {
			rows = append(rows, []string{m.Label, m.ID})
		}
		output.Table(os.Stdout, []string{"LABEL", "ID"}, rows)
		return nil
	},
}

var milestoneCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a milestone",
	RunE: func(cmd *cobra.Command, args []string) error {
		label, _ := cmd.Flags().GetString("label")
		if label == "" {
			return fmt.Errorf("--label is required")
		}
		rc, cr, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(msProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		var td int64
		if msTargetDate != "" {
			d, perr := time.Parse("2006-01-02", msTargetDate)
			if perr != nil {
				return fmt.Errorf("--target-date must be YYYY-MM-DD: %w", perr)
			}
			td = d.UnixMilli()
		}
		id, err := createMilestone(cmd.Context(), rc, pr.ID, cr.Account, label, td)
		if err != nil {
			return err
		}
		_ = cache.Update(func(c *cache.Cache) {
			c.Milestones = append(c.Milestones, cache.MilestoneRec{ID: id, Project: pr.Identifier, Label: label})
		})
		fmt.Printf("created milestone %s (%s)\n", label, id)
		return nil
	},
}

func listMilestones(ctx context.Context, rc *huly.RestClient, projectRef string) ([]huly.Milestone, error) {
	var ms []huly.Milestone
	_, err := rc.FindAll(ctx, huly.ClassMilestone, map[string]any{"space": projectRef}, nil, &ms)
	if err != nil {
		return nil, mapAuthErr(err)
	}
	return ms, nil
}

func createMilestone(ctx context.Context, rc *huly.RestClient, projectRef, account, label string, targetDate int64) (string, error) {
	attrs := map[string]any{"label": label, "status": 0}
	if targetDate != 0 {
		attrs["targetDate"] = targetDate
	}
	tx := huly.NewCreateDocTx(huly.ClassMilestone, projectRef, attrs, account, nowMillis())
	if err := rc.Tx(ctx, tx); err != nil {
		return "", mapAuthErr(err)
	}
	return tx["objectId"].(string), nil
}

func init() {
	milestoneCmd.PersistentFlags().StringVar(&msProject, "project", "", "project identifier")
	_ = milestoneCmd.RegisterFlagCompletionFunc("project", completeProjects)
	milestoneCreateCmd.Flags().String("label", "", "milestone label")
	milestoneCreateCmd.Flags().StringVar(&msTargetDate, "target-date", "", "target date YYYY-MM-DD")
	milestoneCmd.AddCommand(milestoneListCmd, milestoneCreateCmd)
	rootCmd.AddCommand(milestoneCmd)
}
