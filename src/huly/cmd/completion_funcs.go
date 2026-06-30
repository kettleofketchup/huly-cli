package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/cache"
)

func filterPrefix(cands []string, prefix string) []string {
	if prefix == "" {
		return cands
	}
	out := cands[:0:0]
	for _, c := range cands {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func completeProjects(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	c, _ := cache.Load()
	var out []string
	for _, p := range c.Projects {
		out = append(out, p.Identifier)
	}
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeIssues(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	c, _ := cache.Load()
	var out []string
	for _, i := range c.Issues {
		out = append(out, i.Identifier)
	}
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeComponents(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	c, _ := cache.Load()
	var out []string
	for _, x := range c.Components {
		out = append(out, x.Label)
	}
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeMilestones(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	c, _ := cache.Load()
	var out []string
	for _, m := range c.Milestones {
		out = append(out, m.Label)
	}
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completeStatuses(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	c, _ := cache.Load()
	seen := map[string]struct{}{}
	var out []string
	for _, s := range c.Statuses {
		if _, ok := seen[s.Name]; ok {
			continue
		}
		seen[s.Name] = struct{}{}
		out = append(out, s.Name)
	}
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func completePriorities(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	out := []string{"NoPriority", "Urgent", "High", "Medium", "Low"}
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}
