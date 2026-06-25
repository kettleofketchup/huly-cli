package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level configuration struct.
// Add fields here and they will be automatically available in:
//   - config YAML files
//   - environment variables (prefixed with HULY_)
//   - JSON schema (run: just config::schema)
type Config struct {
	Log LogConfig `yaml:"log" json:"log" jsonschema:"title=Logging Configuration,description=Configure log output"`
}

// LogConfig controls logging behavior.
type LogConfig struct {
	Level  string `yaml:"level"  json:"level"  jsonschema:"enum=debug,enum=info,enum=warn,enum=error,default=info,description=Log verbosity level"`
	Format string `yaml:"format" json:"format" jsonschema:"enum=text,enum=json,default=text,description=Log output format"`
}

// Defaults registers default values with viper.
func Defaults() {
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "text")
}

// SetupEnv configures viper to read environment variables.
func SetupEnv() {
	viper.SetEnvPrefix("HULY")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}

// Load unmarshals viper config into a Config struct.
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
