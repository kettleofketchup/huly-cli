package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
"github.com/kettleofketchup/huly-cli/src/huly/config"
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

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	AppConfig = cfg
}
