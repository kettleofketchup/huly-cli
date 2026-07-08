package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/skills"
	"github.com/kettleofketchup/huly-cli/src/huly/version"
)

// errCancelled marks a user-aborted form (Ctrl-C/Esc). Command RunE wrappers
// map it to a clean "cancelled" message and exit 0 (nothing was written).
var errCancelled = errors.New("cancelled")

// runForm runs a huh form and normalizes a user abort to errCancelled.
func runForm(f *huh.Form) error {
	if err := f.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errCancelled
		}
		return err
	}
	return nil
}

// silenceCancel converts errCancelled into a clean stderr message + nil (exit
// 0). Command RunE funcs wrap their TUI-driven results with this.
func silenceCancel(err error) error {
	if errors.Is(err, errCancelled) {
		fmt.Fprintln(os.Stderr, "cancelled")
		return nil
	}
	return err
}

// wantInteractive is the pure interactivity decision: a huh form may run only
// when both stdin and stderr are TTYs (huh renders to stderr) and the user did
// not pass --no-interactive.
func wantInteractive(noInteractive, stdinTTY, stderrTTY bool) bool {
	return !noInteractive && stdinTTY && stderrTTY
}

// isInteractive resolves wantInteractive against the real process TTYs.
func isInteractive(noInteractive bool) bool {
	return wantInteractive(noInteractive,
		term.IsTerminal(int(os.Stdin.Fd())),
		term.IsTerminal(int(os.Stderr.Fd())))
}

// pickAgents shows a pre-checked multi-select of the present agents and returns
// the chosen ones. An empty selection returns an error (nothing to do).
func pickAgents(present []skills.Agent) ([]skills.Agent, error) {
	opts := make([]huh.Option[string], 0, len(present))
	for _, a := range present {
		opts = append(opts, huh.NewOption(a.Label, a.ID).Selected(true))
	}
	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Install into which agents?").
			Options(opts...).
			Value(&chosen),
	))
	if err := runForm(form); err != nil {
		return nil, err
	}
	byID := map[string]skills.Agent{}
	for _, a := range present {
		byID[a.ID] = a
	}
	var out []skills.Agent
	for _, id := range chosen {
		out = append(out, byID[id])
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no agents selected")
	}
	return out, nil
}

// confirmApply asks for a yes/no before a mutating action; returns the choice.
func confirmApply(action string, skillNames, agentIDs []string) (bool, error) {
	ok := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("%s %d skill(s) into %d agent(s)?", action, len(skillNames), len(agentIDs))).
			Affirmative("Yes").
			Negative("No").
			Value(&ok),
	))
	if err := runForm(form); err != nil {
		return false, err
	}
	return ok, nil
}

// runDashboard is the bare `huly skills` interactive flow: choose skills, then
// agents, then an action, then apply and print the result tokens.
func runDashboard() error {
	cat, err := skills.Catalog()
	if err != nil {
		return err
	}
	detected, err := skills.Detect()
	if err != nil {
		return err
	}
	present := presentAgents(detected)
	if len(present) == 0 {
		return fmt.Errorf("%s", noAgentsMessage(detected))
	}

	skillOpts := make([]huh.Option[string], 0, len(cat))
	for _, s := range cat {
		skillOpts = append(skillOpts, huh.NewOption(s.Name, s.Name).Selected(true))
	}
	agentOpts := make([]huh.Option[string], 0, len(present))
	for _, a := range present {
		agentOpts = append(agentOpts, huh.NewOption(a.Label, a.ID).Selected(true))
	}

	var chosenSkills, chosenAgents []string
	var action string
	form := huh.NewForm(
		huh.NewGroup(huh.NewMultiSelect[string]().Title("Skills").Options(skillOpts...).Value(&chosenSkills)),
		huh.NewGroup(huh.NewMultiSelect[string]().Title("Agents").Options(agentOpts...).Value(&chosenAgents)),
		huh.NewGroup(huh.NewSelect[string]().Title("Action").
			Options(huh.NewOption("Install", "install"), huh.NewOption("Update", "update"), huh.NewOption("Uninstall", "uninstall")).
			Value(&action)),
	)
	if err := runForm(form); err != nil {
		return err
	}
	if len(chosenSkills) == 0 || len(chosenAgents) == 0 {
		fmt.Fprintln(os.Stderr, "nothing selected")
		return nil
	}

	// Always confirm before applying (the dashboard has no --yes flag, and the
	// action may be a destructive uninstall).
	ok, err := confirmApply(action, chosenSkills, chosenAgents)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "cancelled")
		return nil
	}

	// Resolve selections back to engine types and apply.
	byAgent := map[string]skills.Agent{}
	for _, a := range present {
		byAgent[a.ID] = a
	}
	opts := skills.InstallOpts{CurrentVersion: version.Version, Force: skillsForce, DryRun: skillsDryRun}
	var results []skills.Result
	failed := false
	for _, name := range chosenSkills {
		sk, ok := skills.Get(name)
		if !ok {
			continue
		}
		for _, id := range chosenAgents {
			ag := byAgent[id]
			var r skills.Result
			var e error
			switch action {
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
	if err := renderResults(os.Stdout, results, false); err != nil {
		return err
	}
	return exitError(results, failed, skillsFailOnConflict)
}
