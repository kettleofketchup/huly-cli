package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/cache"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

var cacheProject string

var cacheCmd = &cobra.Command{Use: "cache", Short: "Manage the local completion cache"}

var cacheSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Refresh the local cache from Huly",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc, _, err := newClient()
		if err != nil {
			return err
		}
		c, err := syncCache(cmd.Context(), rc, cacheProject)
		if err != nil {
			return err
		}
		if err := cache.Save(c); err != nil {
			return err
		}
		fmt.Printf("cache synced: %d projects, %d issues, %d components, %d milestones, %d statuses\n",
			len(c.Projects), len(c.Issues), len(c.Components), len(c.Milestones), len(c.Statuses))
		return nil
	},
}

// syncCache fetches tracker entities and builds a fresh Cache. If projectFilter
// is non-empty, only that project's children are synced.
func syncCache(ctx context.Context, rc *huly.RestClient, projectFilter string) (cache.Cache, error) {
	var out cache.Cache
	out.SyncedAt = nowMillis()

	var projs []huly.Project
	if _, err := rc.FindAll(ctx, huly.ClassProject, nil, nil, &projs); err != nil {
		return out, mapAuthErr(err)
	}
	// id -> identifier map for child records.
	idToIdent := map[string]string{}
	for _, p := range projs {
		if projectFilter != "" && p.Identifier != projectFilter {
			continue
		}
		idToIdent[p.ID] = p.Identifier
		out.Projects = append(out.Projects, cache.ProjectRec{ID: p.ID, Identifier: p.Identifier, Name: p.Name})
	}

	for pid, ident := range idToIdent {
		var issues []huly.Issue
		if _, err := rc.FindAll(ctx, huly.ClassIssue, map[string]any{"space": pid}, nil, &issues); err != nil {
			return out, mapAuthErr(err)
		}
		for _, is := range issues {
			out.Issues = append(out.Issues, cache.IssueRec{ID: is.ID, Project: ident, Identifier: is.Identifier, Title: is.Title})
		}
		var comps []huly.Component
		if _, err := rc.FindAll(ctx, huly.ClassComponent, map[string]any{"space": pid}, nil, &comps); err != nil {
			return out, mapAuthErr(err)
		}
		for _, c := range comps {
			out.Components = append(out.Components, cache.ComponentRec{ID: c.ID, Project: ident, Label: c.Label})
		}
		var mss []huly.Milestone
		if _, err := rc.FindAll(ctx, huly.ClassMilestone, map[string]any{"space": pid}, nil, &mss); err != nil {
			return out, mapAuthErr(err)
		}
		for _, m := range mss {
			out.Milestones = append(out.Milestones, cache.MilestoneRec{ID: m.ID, Project: ident, Label: m.Label})
		}
		var sts []huly.IssueStatus
		if _, err := rc.FindAll(ctx, huly.ClassIssueStatus, map[string]any{"space": pid}, nil, &sts); err != nil {
			return out, mapAuthErr(err)
		}
		for _, s := range sts {
			out.Statuses = append(out.Statuses, cache.StatusRec{ID: s.ID, Project: ident, Name: s.Name, Category: s.Category})
		}
	}
	return out, nil
}

func init() {
	cacheSyncCmd.Flags().StringVar(&cacheProject, "project", "", "limit sync to one project identifier")
	_ = cacheSyncCmd.RegisterFlagCompletionFunc("project", completeProjects)
	cacheCmd.AddCommand(cacheSyncCmd)
	rootCmd.AddCommand(cacheCmd)
}
