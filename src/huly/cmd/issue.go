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

type issueOpts struct {
	Description string
	Status      string
	Priority    string
	Assignee    string
	Component   string
	Milestone   string
}

var (
	issueProject    string
	issueOptsV      issueOpts
	issueListStatus string // dedicated var for the `list` status FILTER (not the create/update field)
)

var issueCmd = &cobra.Command{Use: "issue", Short: "Manage issues"}

var issueListCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues in a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, _, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(issueProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		query := map[string]any{"space": pr.ID}
		if issueListStatus != "" {
			sid, serr := resolveStatusRef(cmd.Context(), rc, pr.ID, issueListStatus)
			if serr != nil {
				return serr
			}
			query["status"] = sid
		}
		var issues []huly.Issue
		_, err = rc.FindAll(cmd.Context(), huly.ClassIssue, query, nil, &issues)
		if err != nil {
			return mapAuthErr(err)
		}
		if viper.GetString("output") == "json" {
			return output.JSON(os.Stdout, issues)
		}
		rows := make([][]string, 0, len(issues))
		for _, is := range issues {
			rows = append(rows, []string{is.Identifier, is.Title, huly.Priority(is.Priority).String()})
		}
		output.Table(os.Stdout, []string{"ID", "TITLE", "PRIORITY"}, rows)
		return nil
	},
}

var issueViewCmd = &cobra.Command{
	Use:               "view <ID>",
	Short:             "Show one issue",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeIssues,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, _, err := newClient()
		if err != nil {
			return err
		}
		var issues []huly.Issue
		_, err = rc.FindAll(cmd.Context(), huly.ClassIssue, map[string]any{"identifier": args[0]}, nil, &issues)
		if err != nil {
			return mapAuthErr(err)
		}
		if len(issues) == 0 {
			return fmt.Errorf("issue %q not found (hint: run `huly cache sync`)", args[0])
		}
		is := issues[0]
		if viper.GetString("output") == "json" {
			return output.JSON(os.Stdout, is)
		}
		output.Table(os.Stdout,
			[]string{"ID", "TITLE", "PRIORITY", "STATUS"},
			[][]string{{is.Identifier, is.Title, huly.Priority(is.Priority).String(), is.Status}})
		return nil
	},
}

var issueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an issue",
	RunE: func(cmd *cobra.Command, args []string) error {
		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			return fmt.Errorf("--title is required")
		}
		rc, cr, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(issueProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		id, err := createIssue(cmd.Context(), rc, pr.ID, cr.Account, title, issueOptsV)
		if err != nil {
			return err
		}
		fmt.Printf("created issue %q (%s)\n", title, id)
		return nil
	},
}

var issueUpdateCmd = &cobra.Command{
	Use:               "update <ID>",
	Short:             "Update an issue",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: completeIssues,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, cr, err := newClient()
		if err != nil {
			return err
		}
		proj, err := resolveProject(issueProject)
		if err != nil {
			return err
		}
		pr, err := resolveProjectRef(cmd.Context(), rc, proj)
		if err != nil {
			return err
		}
		// Find the target issue by identifier.
		var issues []huly.Issue
		_, err = rc.FindAll(cmd.Context(), huly.ClassIssue, map[string]any{"identifier": args[0]}, nil, &issues)
		if err != nil {
			return mapAuthErr(err)
		}
		if len(issues) == 0 {
			return fmt.Errorf("issue %q not found (hint: run `huly cache sync`)", args[0])
		}
		ops, err := buildIssueOps(cmd.Context(), rc, pr.ID, issueOptsV, cmd)
		if err != nil {
			return err
		}
		if len(ops) == 0 {
			return fmt.Errorf("nothing to update; pass at least one field flag")
		}
		if err := updateIssue(cmd.Context(), rc, pr.ID, issues[0].ID, cr.Account, ops); err != nil {
			return err
		}
		fmt.Printf("updated issue %s\n", args[0])
		return nil
	},
}

// buildIssueOps resolves option flags that were set into a tx operations map.
func buildIssueOps(ctx context.Context, rc *huly.RestClient, projectRef string, o issueOpts, cmd *cobra.Command) (map[string]any, error) {
	ops := map[string]any{}
	if t, _ := cmd.Flags().GetString("title"); t != "" {
		ops["title"] = t
	}
	if o.Description != "" {
		ops["description"] = o.Description
	}
	if o.Priority != "" {
		p, ok := huly.PriorityFromName(o.Priority)
		if !ok {
			return nil, fmt.Errorf("unknown priority %q", o.Priority)
		}
		ops["priority"] = int(p)
	}
	if o.Status != "" {
		sid, err := resolveStatusRef(ctx, rc, projectRef, o.Status)
		if err != nil {
			return nil, err
		}
		ops["status"] = sid
	}
	if o.Component != "" {
		cid, err := resolveComponentRef(ctx, rc, projectRef, o.Component)
		if err != nil {
			return nil, err
		}
		ops["component"] = cid
	}
	if o.Milestone != "" {
		mid, err := resolveMilestoneRef(ctx, rc, projectRef, o.Milestone)
		if err != nil {
			return nil, err
		}
		ops["milestone"] = mid
	}
	if o.Assignee != "" {
		ops["assignee"] = o.Assignee
	}
	return ops, nil
}

func createIssue(ctx context.Context, rc *huly.RestClient, projectRef, account, title string, o issueOpts) (string, error) {
	attrs := map[string]any{"title": title, "priority": int(huly.NoPriority)}
	if o.Description != "" {
		attrs["description"] = o.Description
	}
	if o.Priority != "" {
		p, ok := huly.PriorityFromName(o.Priority)
		if !ok {
			return "", fmt.Errorf("unknown priority %q", o.Priority)
		}
		attrs["priority"] = int(p)
	}
	if o.Status != "" {
		sid, err := resolveStatusRef(ctx, rc, projectRef, o.Status)
		if err != nil {
			return "", err
		}
		attrs["status"] = sid
	}
	if o.Component != "" {
		cid, err := resolveComponentRef(ctx, rc, projectRef, o.Component)
		if err != nil {
			return "", err
		}
		attrs["component"] = cid
	}
	if o.Milestone != "" {
		mid, err := resolveMilestoneRef(ctx, rc, projectRef, o.Milestone)
		if err != nil {
			return "", err
		}
		attrs["milestone"] = mid
	}
	if o.Assignee != "" {
		attrs["assignee"] = o.Assignee
	}
	tx := huly.NewCreateIssueTx(projectRef, attrs, account, nowMillis())
	if err := rc.Tx(ctx, tx); err != nil {
		return "", mapAuthErr(err)
	}
	id := tx["objectId"].(string)
	_ = cache.Update(func(c *cache.Cache) {
		c.Issues = append(c.Issues, cache.IssueRec{ID: id, Title: title})
	})
	return id, nil
}

func updateIssue(ctx context.Context, rc *huly.RestClient, projectRef, issueID, account string, ops map[string]any) error {
	tx := huly.NewUpdateDocTx(huly.ClassIssue, projectRef, issueID, ops, account, nowMillis())
	if err := rc.Tx(ctx, tx); err != nil {
		return mapAuthErr(err)
	}
	return nil
}

func registerIssueFlags(c *cobra.Command) {
	c.Flags().String("title", "", "issue title")
	c.Flags().StringVar(&issueOptsV.Description, "description", "", "description")
	c.Flags().StringVar(&issueOptsV.Status, "status", "", "status name")
	c.Flags().StringVar(&issueOptsV.Priority, "priority", "", "priority: NoPriority|Urgent|High|Medium|Low")
	c.Flags().StringVar(&issueOptsV.Assignee, "assignee", "", "assignee ref")
	c.Flags().StringVar(&issueOptsV.Component, "component", "", "component label")
	c.Flags().StringVar(&issueOptsV.Milestone, "milestone", "", "milestone label")
	_ = c.RegisterFlagCompletionFunc("status", completeStatuses)
	_ = c.RegisterFlagCompletionFunc("priority", completePriorities)
	_ = c.RegisterFlagCompletionFunc("component", completeComponents)
	_ = c.RegisterFlagCompletionFunc("milestone", completeMilestones)
}

func init() {
	issueCmd.PersistentFlags().StringVar(&issueProject, "project", "", "project identifier")
	_ = issueCmd.RegisterFlagCompletionFunc("project", completeProjects)
	issueListCmd.Flags().StringVar(&issueListStatus, "status", "", "filter by status name")
	_ = issueListCmd.RegisterFlagCompletionFunc("status", completeStatuses)
	registerIssueFlags(issueCreateCmd)
	registerIssueFlags(issueUpdateCmd)
	issueCmd.AddCommand(issueListCmd, issueViewCmd, issueCreateCmd, issueUpdateCmd)
	rootCmd.AddCommand(issueCmd)
}
