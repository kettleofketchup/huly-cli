package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
"github.com/kettleofketchup/huly-cli/src/huly/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit hash, and build date of huly.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("huly %s\n", version.Version)
		fmt.Printf("  Commit:     %s\n", version.Commit)
		fmt.Printf("  Built:      %s\n", version.BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
