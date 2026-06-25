package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
"github.com/kettleofketchup/huly-cli/src/huly/config"
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

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configSchemaCmd)
	rootCmd.AddCommand(configCmd)
}
