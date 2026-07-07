package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/kettleofketchup/huly-cli/src/huly/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View, inspect, and validate huly configuration.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		out, err := yaml.Marshal(AppConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling config: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(string(out))
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Run: func(cmd *cobra.Command, args []string) {
		f := viper.ConfigFileUsed()
		if f == "" {
			fmt.Println("No config file loaded (using defaults)")
		} else {
			fmt.Println(f)
		}
	},
}

var configSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Output JSON Schema for config file",
	Run: func(cmd *cobra.Command, args []string) {
		r := new(jsonschema.Reflector)
		schema := r.Reflect(&config.Config{})
		schema.Title = "huly configuration"
		schema.Description = "Configuration file schema for huly"
		out, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating schema: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	},
}

// validConfigKeys is the allowlist of keys `config set` may write, so typos
// fail loudly instead of writing a dead key.
var validConfigKeys = map[string]bool{
	"server.url":       true,
	"login.email":      true,
	"login.workspace":  true,
	"defaults.project": true,
	"output":           true,
	"log.level":        true,
	"log.format":       true,
}

// resolveConfigPath picks the file config writes should target.
func resolveConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	if used := viper.ConfigFileUsed(); used != "" {
		return used
	}
	return filepath.Join("config", "huly.yaml")
}

// writeConfigValues merges kv into the YAML file at path using a fresh viper
// instance (NOT the global one) so defaults/env are never serialized to disk.
func writeConfigValues(path string, kv map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing config %s: %w", path, err)
	}
	for k, val := range kv {
		v.Set(k, val)
	}
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a single config value (e.g. login.email me@corp.com)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		if !validConfigKeys[key] {
			keys := make([]string, 0, len(validConfigKeys))
			for k := range validConfigKeys {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return fmt.Errorf("unknown config key %q; valid keys: %s", key, strings.Join(keys, ", "))
		}
		path := resolveConfigPath()
		if err := writeConfigValues(path, map[string]string{key: value}); err != nil {
			return err
		}
		fmt.Printf("set %s in %s\n", key, path)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configSchemaCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}
