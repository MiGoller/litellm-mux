package cmd

import (
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage LiteLLM proxy models",
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(lsCmd)
	modelsCmd.AddCommand(rmCmd)
	modelsCmd.AddCommand(copyCmd)
}
