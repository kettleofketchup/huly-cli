package creds

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// ErrNoCredentials indicates no token from env or file.
var ErrNoCredentials = errors.New("no huly credentials: run `huly login` or `huly auth set-token`")

// Credentials are the workspace connection secrets.
type Credentials struct {
	Endpoint  string `yaml:"endpoint"  mapstructure:"endpoint"`
	Workspace string `yaml:"workspace" mapstructure:"workspace"`
	Token     string `yaml:"token"     mapstructure:"token"`
	Account   string `yaml:"account"   mapstructure:"account"`
}

// Path returns the credentials file path under the user config dir.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "huly", "credentials.yaml"), nil
}

// Load reads credentials, letting HULY_* env vars override the file.
func Load() (Credentials, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	p, err := Path()
	if err != nil {
		return Credentials{}, err
	}
	v.SetConfigFile(p)
	_ = v.ReadInConfig() // missing file is fine; env may supply values

	var c Credentials
	if err := v.Unmarshal(&c); err != nil {
		return Credentials{}, err
	}
	// Env overrides (explicit, not viper-bound, so empty file keys are fine).
	if e := os.Getenv("HULY_ENDPOINT"); e != "" {
		c.Endpoint = e
	}
	if w := os.Getenv("HULY_WORKSPACE"); w != "" {
		c.Workspace = w
	}
	if t := os.Getenv("HULY_TOKEN"); t != "" {
		c.Token = t
	}
	if c.Token == "" {
		return c, ErrNoCredentials
	}
	return c, nil
}

// Save writes credentials to the 0600 file, creating the directory.
func Save(c Credentials) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	v := viper.New()
	v.Set("endpoint", c.Endpoint)
	v.Set("workspace", c.Workspace)
	v.Set("token", c.Token)
	v.Set("account", c.Account)
	if err := v.WriteConfigAs(p); err != nil {
		return err
	}
	return os.Chmod(p, 0o600)
}

// Clear removes the credentials file (no error if absent).
func Clear() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
