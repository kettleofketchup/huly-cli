package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status is the outcome of an install/update/uninstall on one (skill, agent).
type Status string

const (
	StatusInstalled Status = "installed"
	StatusUpdated   Status = "updated"
	StatusRepaired  Status = "repaired"
	StatusUpToDate  Status = "up-to-date"
	StatusConflict  Status = "conflict"
	StatusRemoved   Status = "removed"
	StatusSkipped   Status = "skipped"
)

// Result reports what happened for one (skill, agent) target.
type Result struct {
	Skill  string
	Agent  string
	Path   string
	Status Status
	Reason string // "foreign","unreadable","modified","adopted","absent", ""
}

// InstallOpts controls install/update/uninstall behavior.
type InstallOpts struct {
	CurrentVersion string // stamped as provenance; from version.Version at the CLI layer
	Force          bool   // override conflict guards (backs up first)
	DryRun         bool   // classify + report, write nothing
}

// Install ensures the shipped skill is present and current for the agent.
// A fresh dest is installed; an up-to-date one is left alone (idempotent).
// Force overrides the conflict guards (backing up first); on an already
// up-to-date tree it is a no-op, since up-to-date proves the body+siblings are
// byte-identical to the embedded tree.
func Install(sk Skill, ag Agent, o InstallOpts) (Result, error) {
	return apply(sk, ag, o, false)
}

// Update refreshes an already-installed, huly-owned skill that is behind.
// An absent skill is skipped (not installed).
func Update(sk Skill, ag Agent, o InstallOpts) (Result, error) {
	return apply(sk, ag, o, true)
}

func apply(sk Skill, ag Agent, o InstallOpts, updateOnly bool) (Result, error) {
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	res := Result{Skill: sk.Name, Agent: ag.ID, Path: dest}

	if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
		if updateOnly {
			res.Status, res.Reason = StatusSkipped, "absent"
			return res, nil
		}
		return finish(sk, dest, o, StatusInstalled, "", res)
	}

	md := filepath.Join(dest, "SKILL.md")
	raw, readErr := os.ReadFile(md)
	if readErr != nil {
		// dest exists but SKILL.md missing/unreadable -> can't prove ours.
		return conflictOrForce(sk, dest, o, "unreadable", res)
	}
	fm, perr := Parse(raw)
	if perr != nil {
		return conflictOrForce(sk, dest, o, "unreadable", res)
	}
	if fm.ManagedBy != "huly-cli" {
		return conflictOrForce(sk, dest, o, "foreign", res)
	}

	// Ours. Gate on content hash.
	emb, err := sk.contentHash()
	if err != nil {
		return res, err
	}
	onDisk, err := ContentHash(os.DirFS(dest))
	if err != nil {
		return res, err
	}

	if fm.ContentHash == "" {
		// Ours but pre-hash (an older huly stamped no content_hash). Adopt
		// WITHOUT clobbering: if the body already matches the embedded tree,
		// just stamp content_hash (hash-neutral, no copy). If it diverges, the
		// state is ambiguous (old shipped content vs a user edit) -> treat as
		// modified so --force backs up before overwriting.
		if onDisk == emb {
			if !o.DryRun {
				if err := restampVersion(md, raw, o.CurrentVersion, emb); err != nil {
					return res, err
				}
			}
			res.Status, res.Reason = StatusUpdated, "adopted"
			return res, nil
		}
		return conflictOrForce(sk, dest, o, "modified", res)
	}
	if onDisk == fm.ContentHash {
		// Unmodified.
		if emb == fm.ContentHash {
			// Up to date. Refresh provenance version only (hash-neutral).
			if fm.Version != o.CurrentVersion && !o.DryRun {
				if err := restampVersion(md, raw, o.CurrentVersion, fm.ContentHash); err != nil {
					return res, err
				}
			}
			res.Status = StatusUpToDate
			return res, nil
		}
		// Shipped content changed -> update.
		return finish(sk, dest, o, StatusUpdated, "", res)
	}
	// On-disk diverges from stored -> user-edited.
	return conflictOrForce(sk, dest, o, "modified", res)
}

// conflictOrForce reports a conflict, or (with Force) backs up + overwrites.
func conflictOrForce(sk Skill, dest string, o InstallOpts, reason string, res Result) (Result, error) {
	if !o.Force {
		res.Status, res.Reason = StatusConflict, reason
		return res, nil
	}
	if !o.DryRun {
		if err := backup(dest); err != nil {
			return res, err
		}
	}
	status := StatusUpdated
	if reason == "unreadable" || reason == "foreign" {
		status = StatusRepaired
	}
	return finish(sk, dest, o, status, reason, res)
}

// finish writes the tree (unless DryRun) and stamps the result.
func finish(sk Skill, dest string, o InstallOpts, status Status, reason string, res Result) (Result, error) {
	if !o.DryRun {
		if err := writeTree(sk, dest, o.CurrentVersion); err != nil {
			return res, err
		}
	}
	res.Status, res.Reason = status, reason
	return res, nil
}

// backup renames dest aside so it is recoverable after a forced overwrite.
// UnixNano avoids a same-instant collision between two backups of one dest.
func backup(dest string) error {
	bak := fmt.Sprintf("%s.bak-%d", dest, time.Now().UnixNano())
	return os.Rename(dest, bak)
}

// restampVersion rewrites only the SKILL.md to refresh the provenance version,
// keeping the same content_hash (hash-neutral, no tree copy).
func restampVersion(md string, raw []byte, version, hash string) error {
	out, err := Stamp(raw, "huly-cli", version, hash)
	if err != nil {
		return err
	}
	return os.WriteFile(md, out, 0o644)
}

// Uninstall removes a skill from an agent, but only one huly owns unless Force.
// A foreign/unreadable dir removed under Force is backed up (never destroyed),
// mirroring install --force and the "never destroy unproven content" rule.
func Uninstall(sk Skill, ag Agent, o InstallOpts) (Result, error) {
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	res := Result{Skill: sk.Name, Agent: ag.ID, Path: dest}

	if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
		res.Status, res.Reason = StatusSkipped, "absent"
		return res, nil
	}

	ours := false
	reason := "unreadable" // no/unparseable SKILL.md
	if raw, err := os.ReadFile(filepath.Join(dest, "SKILL.md")); err == nil {
		if fm, perr := Parse(raw); perr == nil {
			if fm.ManagedBy == "huly-cli" {
				ours = true
			} else {
				reason = "foreign"
			}
		}
	}
	if !ours && !o.Force {
		res.Status, res.Reason = StatusConflict, reason
		return res, nil
	}
	if !o.DryRun {
		if ours {
			if err := os.RemoveAll(dest); err != nil {
				return res, err
			}
		} else {
			// Force-removing a dir we cannot prove is ours: back it up rather
			// than destroy it.
			if err := backup(dest); err != nil {
				return res, err
			}
		}
	}
	res.Status = StatusRemoved
	if !ours {
		res.Reason = reason
	}
	return res, nil
}
