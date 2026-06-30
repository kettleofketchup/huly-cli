package cache

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type ProjectRec struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
}
type ComponentRec struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Label   string `json:"label"`
}
type MilestoneRec struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Label   string `json:"label"`
}
type StatusRec struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
}
type IssueRec struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// Cache is the local read-through mirror of Huly tracker data.
type Cache struct {
	Projects   []ProjectRec   `json:"projects"`
	Components []ComponentRec `json:"components"`
	Milestones []MilestoneRec `json:"milestones"`
	Statuses   []StatusRec    `json:"statuses"`
	Issues     []IssueRec     `json:"issues"`
	SyncedAt   int64          `json:"syncedAt"`
}

func dir() (string, error) {
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "huly"), nil
}

// Path returns the cache file path.
func Path() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cache.json"), nil
}

func lockPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "cache.lock"), nil
}

// Load reads the cache; a missing file yields an empty Cache with nil error.
func Load() (Cache, error) {
	p, err := Path()
	if err != nil {
		return Cache{}, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return Cache{}, nil
	}
	if err != nil {
		return Cache{}, err
	}
	var c Cache
	if err := json.Unmarshal(b, &c); err != nil {
		// Corrupt cache behaves like an empty one (self-heal on next sync).
		return Cache{}, nil
	}
	return c, nil
}

// Save writes the cache atomically (tmp + rename).
func Save(c Cache) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Update performs a locked read-modify-write.
func Update(fn func(*Cache)) error {
	lp, err := lockPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lp), 0o700); err != nil {
		return err
	}
	fl := flock.New(lp)
	if err := fl.Lock(); err != nil {
		return err
	}
	defer func() { _ = fl.Unlock() }()

	c, err := Load()
	if err != nil {
		return err
	}
	fn(&c)
	return Save(c)
}
