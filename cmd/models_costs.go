package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/MiGoller/litellm-mux/internal/client"
	"github.com/spf13/cobra"
)

var (
	flagCostInput     float64
	flagCostOutput    float64
	flagCostCacheRead float64
	flagCostCacheWrite float64
	flagCostHasInput  bool
	flagCostHasOutput bool
	flagCostHasCR     bool
	flagCostHasCW     bool
	flagCostDryRun    bool
	flagCostYes       bool
)

var costsModelCmd = &cobra.Command{
	Use:   "costs",
	Short: "Manage model pricing / costs",
	Long: `Manage per-token model pricing on LiteLLM.

Available subcommands:
  set   Set pricing for models (input, output, cache read/write)

Parameters for 'set':
  --input <float>       Input cost per 1M tokens ($)
  --output <float>      Output cost per 1M tokens ($)
  --cache-read <float>  Cache read input cost per 1M tokens ($)
  --cache-write <float> Cache write / creation input cost per 1M tokens ($)

Examples:
  litellm-mux models costs set --help
  litellm-mux models -f "provider:gemini" costs set --input 0.15 --output 0.60 --cache-read 0.03 --cache-write 0.07`,
}

var costsSetCmd = &cobra.Command{
	Use:   "set [models...]",
	Short: "Set pricing (input, output, cache read/write) for models (considering -f filters)",
	Long: `Set per-token pricing for models on the LiteLLM proxy gateway.
Costs are specified per 1M tokens (e.g. --input 0.15 for $0.15 per 1M tokens).

Examples:
  litellm-mux models -f "provider:gemini" costs set --input 0.15 --output 0.60 --cache-read 0.03 --cache-write 0.07
  litellm-mux models -f "model:gpt-4" costs set --input 2.50 --output 10.00 -y`,
	Run: func(cmd *cobra.Command, args []string) {
		if !flagCostHasInput && !flagCostHasOutput && !flagCostHasCR && !flagCostHasCW {
		// If flags weren't explicitly set via float64 parsing check, check if any flag was provided
		}
		// Better check if flags were changed or at least one cost is provided
		if flagCostInput == 0 && flagCostOutput == 0 && flagCostCacheRead == 0 && flagCostCacheWrite == 0 {
			// Check if flags were explicitly passed or if user wants to clear / set zero.
			// To be safe, let's allow setting 0 or require at least one flag to be explicitly flagged, 
			// but checking command line flags via cmd.Flags().Changed(...) is cleaner.
		}

		hasChanged := cmd.Flags().Changed("input") || cmd.Flags().Changed("output") || 
		              cmd.Flags().Changed("cache-read") || cmd.Flags().Changed("cache-write")

		if !hasChanged {
			fmt.Fprintln(os.Stderr, "Error: provide at least one cost parameter (--input, --output, --cache-read, --cache-write).")
			os.Exit(1)
		}

		cfg := GetConfig()
		apiClient := client.NewClient(cfg.LiteLLMURL, cfg.MasterKey)

		resp := fetchModels(apiClient)
		selected := selectModels(resp, args)
		if len(selected) == 0 {
			fmt.Println("No matching models found.")
			return
		}

		type plan struct {
			ModelName string
			ModelID   string
			Payload   map[string]interface{}
			Changes   []string
		}

		var plans []plan

		for _, m := range selected {
			modelID, _ := m.ModelInfo["id"].(string)
			if modelID == "" {
				modelID, _ = m.ModelInfo["model_id"].(string)
			}

			// Copy existing litellm_params and model_info
			lpCopy := make(map[string]interface{})
			for k, v := range m.LitellmParams {
				lpCopy[k] = v
			}
			miCopy := make(map[string]interface{})
			for k, v := range m.ModelInfo {
				if k != "id" && k != "db_model" && k != "key" {
					miCopy[k] = v
				}
			}

			var changes []string

			if cmd.Flags().Changed("input") {
				valPerToken := flagCostInput / 1_000_000
				miCopy["input_cost_per_token"] = valPerToken
				lpCopy["input_cost_per_token"] = valPerToken
				changes = append(changes, fmt.Sprintf("INPUT: $%.2f/1M", flagCostInput))
			}
			if cmd.Flags().Changed("output") {
				valPerToken := flagCostOutput / 1_000_000
				miCopy["output_cost_per_token"] = valPerToken
				lpCopy["output_cost_per_token"] = valPerToken
				changes = append(changes, fmt.Sprintf("OUTPUT: $%.2f/1M", flagCostOutput))
			}
			if cmd.Flags().Changed("cache-read") {
				valPerToken := flagCostCacheRead / 1_000_000
				miCopy["cache_read_input_token_cost"] = valPerToken
				lpCopy["cache_read_input_token_cost"] = valPerToken
				changes = append(changes, fmt.Sprintf("CACHE READ: $%.2f/1M", flagCostCacheRead))
			}
			if cmd.Flags().Changed("cache-write") {
				valPerToken := flagCostCacheWrite / 1_000_000
				miCopy["cache_creation_input_token_cost"] = valPerToken
				lpCopy["cache_creation_input_token_cost"] = valPerToken
				changes = append(changes, fmt.Sprintf("CACHE WRITE: $%.2f/1M", flagCostCacheWrite))
			}

			payload := map[string]interface{}{
				"model_info":     miCopy,
				"litellm_params": lpCopy,
			}
			if modelID != "" && modelID != "-" {
				miCopy["id"] = modelID
			}

			plans = append(plans, plan{
				ModelName: enableModelDisplayName(m),
				ModelID:   modelID,
				Payload:   payload,
				Changes:   changes,
			})
		}

		fmt.Println("Planned pricing updates:")
		fmt.Println(strings.Repeat("-", 70))
		for _, p := range plans {
			fmt.Printf("  %s (ID: %s)\n", p.ModelName, p.ModelID)
			for _, ch := range p.Changes {
				fmt.Printf("    - %s\n", ch)
			}
			fmt.Println("    ---")
		}
		fmt.Println(strings.Repeat("-", 70))

		if flagCostDryRun {
			fmt.Println("[Dry-Run] Nothing was changed.")
			return
		}

		if !flagCostYes {
			fmt.Printf("Do you want to update pricing for these %d model(s)? [y/N]: ", len(plans))
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Aborted.")
				return
			}
		}

		for _, p := range plans {
			var result map[string]interface{}
			err := apiClient.Request("POST", "/model/update", p.Payload, &result)
			if err != nil {
				fmt.Printf("Error updating pricing for %s: %v\n", p.ModelName, err)
			} else {
				fmt.Printf("Successfully updated pricing: %s\n", p.ModelName)
			}
		}
	},
}

func init() {
	costsSetCmd.Flags().Float64Var(&flagCostInput, "input", 0, "Input cost per 1M tokens ($)")
	costsSetCmd.Flags().Float64Var(&flagCostOutput, "output", 0, "Output cost per 1M tokens ($)")
	costsSetCmd.Flags().Float64Var(&flagCostCacheRead, "cache-read", 0, "Cache read input cost per 1M tokens ($)")
	costsSetCmd.Flags().Float64Var(&flagCostCacheWrite, "cache-write", 0, "Cache write / creation input cost per 1M tokens ($)")
	costsSetCmd.Flags().BoolVarP(&flagCostDryRun, "dry-run", "n", false, "Show the plan without making changes")
	costsSetCmd.Flags().BoolVarP(&flagCostYes, "yes", "y", false, "Skip confirmation prompt")

	costsModelCmd.AddCommand(costsSetCmd)
	modelsCmd.AddCommand(costsModelCmd)
}
