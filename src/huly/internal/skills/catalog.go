package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"sync"
)

//go:embed all:assets
var assetsFS embed.FS

// Skill is one entry in the embedded catalog.
type Skill struct {
	Name        string
	Description string
	fsPath      string // path within assetsFS, e.g. "assets/huly-issue-tracking"
}

var (
	catalogOnce sync.Once
	catalogVal  []Skill
	catalogErr  error
)

// Catalog returns the embedded skills, parsing the embedded FS once.
func Catalog() ([]Skill, error) {
	catalogOnce.Do(func() { catalogVal, catalogErr = loadCatalog() })
	return catalogVal, catalogErr
}

// loadCatalog walks assets/ and parses each SKILL.md frontmatter, one per
// non-dot directory under assets/. Dot-prefixed entries are ignored.
func loadCatalog() ([]Skill, error) {
	entries, err := fs.ReadDir(assetsFS, "assets")
	if err != nil {
		return nil, fmt.Errorf("read embedded assets: %w", err)
	}
	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := "assets/" + e.Name()
		raw, err := assetsFS.ReadFile(path + "/SKILL.md")
		if err != nil {
			return nil, fmt.Errorf("skill %q has no SKILL.md: %w", e.Name(), err)
		}
		fm, err := Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", e.Name(), err)
		}
		skills = append(skills, Skill{Name: fm.Name, Description: fm.Description, fsPath: path})
	}
	return skills, nil
}

// Get returns the catalog skill with the given name.
func Get(name string) (Skill, bool) {
	cat, err := Catalog()
	if err != nil {
		return Skill{}, false
	}
	for _, s := range cat {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}
