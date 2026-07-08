package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/output"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/skills"
	"github.com/kettleofketchup/huly-cli/src/huly/version"
)

var (
	skillsAgents         string
	skillsAll            bool
	skillsForce          bool
	skillsDryRun         bool
	skillsFailOnConflict bool
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Install and manage huly's embedded agent skills",
	Long: `Install huly's embedded agent skills into the coding agents on your
machine (Claude Code, Codex, opencode, Cursor, Pi).

Run 'huly skills list' to see status, and 'huly skills install --all' to add
them to every detected agent.`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show embedded skills and their install status per agent",
	RunE:  runSkillsList,
}

func runSkillsList(_ *cobra.Command, _ []string) error {
	cat, err := skills.Catalog()
	if err != nil {
		return err
	}
	agents, err := skills.Detect()
	if err != nil {
		return err
	}
	present := presentAgents(agents)

	type listRow struct {
		Skill  string `json:"skill"`
		Agent  string `json:"agent"`
		Status string `json:"status"`
	}

	if len(present) == 0 {
		if viper.GetString("output") == "json" {
			return output.JSON(os.Stdout, []listRow{})
		}
		fmt.Fprintln(os.Stderr, noAgentsMessage(agents))
		return nil
	}

	var rows []listRow
	anyInstalled := false
	for _, sk := range cat {
		for _, ag := range present {
			r, err := skills.Install(sk, ag, skills.InstallOpts{CurrentVersion: version.Version, DryRun: true})
			if err != nil {
				return err
			}
			label := listLabel(r)
			if label != "not installed" {
				anyInstalled = true
			}
			rows = append(rows, listRow{sk.Name, ag.ID, label})
		}
	}

	if viper.GetString("output") == "json" {
		return output.JSON(os.Stdout, rows)
	}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		table = append(table, []string{r.Skill, r.Agent, r.Status})
	}
	output.Table(os.Stdout, []string{"SKILL", "AGENT", "STATUS"}, table)
	if !anyInstalled {
		fmt.Fprintln(os.Stderr, "\nNo skills installed yet. Run `huly skills install --all` to add them.")
	}
	return nil
}

var skillsInstallCmd = &cobra.Command{
	Use:               "install [skill...]",
	Short:             "Install embedded skills into agents",
	ValidArgsFunction: completeSkills,
	RunE:              func(_ *cobra.Command, args []string) error { return runSkillsOp("install", args) },
}

var skillsUpdateCmd = &cobra.Command{
	Use:               "update [skill...]",
	Short:             "Update huly-owned skills that are behind",
	ValidArgsFunction: completeSkills,
	RunE:              func(_ *cobra.Command, args []string) error { return runSkillsOp("update", args) },
}

var skillsUninstallCmd = &cobra.Command{
	Use:               "uninstall [skill...]",
	Short:             "Remove huly-owned skills from agents",
	ValidArgsFunction: completeSkills,
	RunE:              func(_ *cobra.Command, args []string) error { return runSkillsOp("uninstall", args) },
}

// runSkillsOp resolves the target skills and agents, runs the engine op for
// every (skill, agent) pair, renders the results, and returns an error (=>
// non-zero exit) only on a genuine failure or an unforced conflict under
// --fail-on-conflict.
func runSkillsOp(op string, args []string) error {
	sks, err := resolveTargetSkills(args)
	if err != nil {
		return err
	}
	detected, err := skills.Detect()
	if err != nil {
		return err
	}
	agents, err := resolveAgents(detected, skillsAgents, skillsAll)
	if err != nil {
		return err
	}
	opts := skills.InstallOpts{CurrentVersion: version.Version, Force: skillsForce, DryRun: skillsDryRun}

	var results []skills.Result
	failed := false
	for _, sk := range sks {
		for _, ag := range agents {
			var r skills.Result
			var e error
			switch op {
			case "install":
				r, e = skills.Install(sk, ag, opts)
			case "update":
				r, e = skills.Update(sk, ag, opts)
			case "uninstall":
				r, e = skills.Uninstall(sk, ag, opts)
			}
			if e != nil {
				r = skills.Result{Skill: sk.Name, Agent: ag.ID, Status: "error", Reason: e.Error()}
				failed = true
			}
			results = append(results, r)
		}
	}

	if err := renderResults(os.Stdout, results, viper.GetString("output") == "json"); err != nil {
		return err
	}
	return exitError(results, failed, skillsFailOnConflict)
}

func init() {
	for _, c := range []*cobra.Command{skillsInstallCmd, skillsUpdateCmd, skillsUninstallCmd} {
		c.Flags().StringVar(&skillsAgents, "agents", "", "comma-separated agent ids: claude,codex,opencode,cursor,pi")
		c.Flags().BoolVar(&skillsAll, "all", false, "target every detected agent")
		c.Flags().BoolVar(&skillsForce, "force", false, "override conflicts (backs the old dir up first)")
		c.Flags().BoolVar(&skillsDryRun, "dry-run", false, "show what would change; write nothing")
		c.Flags().BoolVar(&skillsFailOnConflict, "fail-on-conflict", false, "exit non-zero if any target conflicts")
		_ = c.RegisterFlagCompletionFunc("agents", completeAgents)
	}
	skillsCmd.AddCommand(skillsListCmd, skillsInstallCmd, skillsUpdateCmd, skillsUninstallCmd)
	rootCmd.AddCommand(skillsCmd)
}
