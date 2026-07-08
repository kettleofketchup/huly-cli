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
	if len(present) == 0 {
		fmt.Fprintln(os.Stderr, noAgentsMessage(agents))
		return nil
	}

	type listRow struct {
		Skill  string `json:"skill"`
		Agent  string `json:"agent"`
		Status string `json:"status"`
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

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	rootCmd.AddCommand(skillsCmd)
}
