package cmd

import (
	"context"
	"fmt"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/cache"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func staleHint(cached bool) string {
	if cached {
		return " (hint: the cache may be stale — run `huly cache sync`)"
	}
	return ""
}

// resolveProjectRef finds a project by identifier (cache first, then live).
func resolveProjectRef(ctx context.Context, rc *huly.RestClient, identifier string) (huly.Project, error) {
	cc, _ := cache.Load()
	cachedID := ""
	for _, p := range cc.Projects {
		if p.Identifier == identifier {
			cachedID = p.ID
			break
		}
	}
	var projs []huly.Project
	_, err := rc.FindAll(ctx, huly.ClassProject, map[string]any{"identifier": identifier}, nil, &projs)
	if err != nil {
		return huly.Project{}, mapAuthErr(err)
	}
	if len(projs) == 0 {
		// Self-healing write: the live lookup found nothing but the id was cached,
		// so the cache is stale — prune the dead project entry (§7).
		if cachedID != "" {
			_ = cache.Update(func(c *cache.Cache) {
				kept := c.Projects[:0]
				for _, p := range c.Projects {
					if p.Identifier != identifier {
						kept = append(kept, p)
					}
				}
				c.Projects = kept
			})
		}
		return huly.Project{}, fmt.Errorf("unknown project %q%s", identifier, staleHint(cachedID != ""))
	}
	return projs[0], nil
}

// resolveStatusRef finds an IssueStatus id by name within a project space.
func resolveStatusRef(ctx context.Context, rc *huly.RestClient, projectRef, name string) (string, error) {
	var ss []huly.IssueStatus
	_, err := rc.FindAll(ctx, huly.ClassIssueStatus,
		map[string]any{"space": projectRef, "name": name}, nil, &ss)
	if err != nil {
		return "", mapAuthErr(err)
	}
	if len(ss) == 0 {
		return "", fmt.Errorf("unknown status %q in project", name)
	}
	return ss[0].ID, nil
}

// resolveComponentRef finds a Component id by label within a project.
func resolveComponentRef(ctx context.Context, rc *huly.RestClient, projectRef, label string) (string, error) {
	var cs []huly.Component
	_, err := rc.FindAll(ctx, huly.ClassComponent,
		map[string]any{"space": projectRef, "label": label}, nil, &cs)
	if err != nil {
		return "", mapAuthErr(err)
	}
	if len(cs) == 0 {
		return "", fmt.Errorf("unknown component %q", label)
	}
	return cs[0].ID, nil
}

// resolveMilestoneRef finds a Milestone id by label within a project.
func resolveMilestoneRef(ctx context.Context, rc *huly.RestClient, projectRef, label string) (string, error) {
	var ms []huly.Milestone
	_, err := rc.FindAll(ctx, huly.ClassMilestone,
		map[string]any{"space": projectRef, "label": label}, nil, &ms)
	if err != nil {
		return "", mapAuthErr(err)
	}
	if len(ms) == 0 {
		return "", fmt.Errorf("unknown milestone %q", label)
	}
	return ms[0].ID, nil
}
