package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/kettleofketchup/huly-cli/src/huly/config"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/output"
)

var (
	cfgFile   string
	AppConfig *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "huly",
	Short: "huly-cli CLI tool",
	Long: `huly-cli is a CLI tool built with Cobra.

Configure it using a config file, environment variables, or flags.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		q, _ := cmd.Flags().GetBool("quiet")
		output.Quiet = q
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config/huly.yaml)")
	rootCmd.PersistentFlags().String("output", "table", "output format: table|json")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "suppress stdout")
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
}

func initConfig() {
	config.Defaults()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("huly")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./config")
		viper.AddConfigPath(".")
	}

	config.SetupEnv()

	if err := viper.ReadInConfig(); err == nil && !isCompletion() {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	cfg, err := config.Load()
	if err != nil {
		if isCompletion() {
			return
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	AppConfig = cfg
}

// isCompletion reports whether this process is a shell-completion invocation.
// Must match BOTH cobra completion entrypoints — bash uses __completeNoDesc.
func isCompletion() bool {
	return len(os.Args) > 1 &&
		(os.Args[1] == cobra.ShellCompRequestCmd ||
			os.Args[1] == cobra.ShellCompNoDescRequestCmd)
}

// resolveProject returns the flag value, else defaults.project, else error.
func resolveProject(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if p := viper.GetString("defaults.project"); p != "" {
		return p, nil
	}
	return "", fmt.Errorf("--project is required (or set defaults.project in huly.yaml)")
}
