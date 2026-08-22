package cmd

import (
	"fmt"
	"os"

	"github.com/MiGoller/litellm-mux/internal/config"
	"github.com/spf13/cobra"
)

var (
	flagURL       string
	flagMasterKey string
)

var rootCmd = &cobra.Command{
	Use:   "litellm-mux",
	Short: "LiteLLM Mux CLI and Proxy Tool",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagURL, "url", "", "LiteLLM API URL (overrides LITELLM_URL and .env)")
	rootCmd.PersistentFlags().StringVar(&flagMasterKey, "master-key", "", "LiteLLM Master Key (overrides MASTER_KEY and .env)")
}

func GetConfig() *config.Config {
	cfg, err := config.LoadConfig(flagURL, flagMasterKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return cfg
}
