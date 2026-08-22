package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/MiGoller/litellm-mux/internal/client"
	"github.com/MiGoller/litellm-mux/internal/filter"
	"github.com/MiGoller/litellm-mux/internal/models"
	"github.com/spf13/cobra"
)

var (
	flagRmDryRun  bool
	flagRmYes     bool
	flagRmFilters []string
)

var rmCmd = &cobra.Command{
	Use:   "rm [models...]",
	Short: "Delete one or more models (by name or filter)",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 && len(flagRmFilters) == 0 {
			fmt.Fprintln(os.Stderr, "Error: provide model names or filters (-f).")
			os.Exit(1)
		}

		cfg := GetConfig()
		apiClient := client.NewClient(cfg.LiteLLMURL, cfg.MasterKey)

		var resp models.ModelInfoResponse
		if err := apiClient.Request("GET", "/model/info", nil, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching models: %v\n", err)
			os.Exit(1)
		}

		if len(resp.Data) == 0 {
			fmt.Println("No models found on the server.")
			return
		}

		var modelsToDelete []models.ModelData
		seenIDs := make(map[string]bool)

		addToDelete := func(m models.ModelData, modelName, modelID string) {
			idKey := modelID
			if idKey == "" {
				idKey = modelName
			}
			if !seenIDs[idKey] {
				modelsToDelete = append(modelsToDelete, m)
				seenIDs[idKey] = true
			}
		}

		getName := func(m models.ModelData) string {
			if m.ModelName != "" {
				return m.ModelName
			}
			if mn, ok := m.ModelInfo["model_name"].(string); ok {
				return mn
			}
			return "N/A"
		}

		getID := func(m models.ModelData) string {
			id, _ := m.ModelInfo["id"].(string)
			if id == "" {
				id, _ = m.ModelInfo["model_id"].(string)
			}
			if id == "" {
				id = "-"
			}
			return id
		}

		// 1. Direct model name matching from positional args
		for _, target := range args {
			found := false
			for _, m := range resp.Data {
				mName := getName(m)
				mID := getID(m)
				if target == mName || target == mID {
					addToDelete(m, mName, mID)
					found = true
				}
			}
			if !found {
				fmt.Printf("Warning: model '%s' was not found on the server.\n", target)
			}
		}

		// 2. Filter matching (-f flags)
		headers := []string{"MODEL NAME", "PROVIDER", "MAX TOKENS", "INPUT / 1M ($)", "OUTPUT / 1M ($)", "MODE", "API BASE", "CREDENTIAL"}
		compiledFilters, _ := filter.ParseFilters(flagRmFilters, headers)

		if len(compiledFilters) > 0 {
			for _, m := range resp.Data {
				modelName := getName(m)

				provider, _ := m.LitellmParams["custom_llm_provider"].(string)
				if provider == "" {
					provider, _ = m.LitellmParams["model"].(string)
				}
				if provider == "" {
					provider = "N/A"
				}

				maxTokens := "-"
				if mt, ok := m.ModelInfo["max_tokens"]; ok && mt != nil {
					maxTokens = fmt.Sprintf("%v", mt)
				} else if mt, ok := m.LitellmParams["max_tokens"]; ok && mt != nil {
					maxTokens = fmt.Sprintf("%v", mt)
				}

				inputStr := "-"
				outputStr := "-"
				if ic, ok := m.ModelInfo["input_cost_per_token"].(float64); ok {
					inputStr = fmt.Sprintf("%.2f", ic*1_000_000)
				}
				if oc, ok := m.ModelInfo["output_cost_per_token"].(float64); ok {
					outputStr = fmt.Sprintf("%.2f", oc*1_000_000)
				}

				mode := "-"
				if md, ok := m.ModelInfo["mode"].(string); ok {
					mode = md
				} else if md, ok := m.LitellmParams["mode"].(string); ok {
					mode = md
				}

				apiBase := "-"
				if ab, ok := m.LitellmParams["api_base"].(string); ok {
					apiBase = ab
				}

				cred := "-"
				if c, ok := m.ModelInfo["litellm_credential_name"].(string); ok {
					cred = c
				} else if c, ok := m.LitellmParams["litellm_credential_name"].(string); ok {
					cred = c
				}

				row := []string{modelName, provider, maxTokens, inputStr, outputStr, mode, apiBase, cred}

				if filter.EvaluateRow(row, compiledFilters) {
					addToDelete(m, modelName, getID(m))
				}
			}
		}

		if len(modelsToDelete) == 0 {
			fmt.Println("No matching models to delete.")
			return
		}

		fmt.Printf("The following %d model(s) would be deleted:\n", len(modelsToDelete))
		fmt.Println(strings.Repeat("-", 60))
		for _, m := range modelsToDelete {
			fmt.Printf(" - %s (ID: %s)\n", getName(m), getID(m))
		}
		fmt.Println(strings.Repeat("-", 60))

		if flagRmDryRun {
			fmt.Println("[Dry-Run] Nothing was deleted.")
			return
		}

		if !flagRmYes {
			fmt.Print("Do you really want to delete these models? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Aborted.")
				return
			}
		}

		for _, m := range modelsToDelete {
			mID := getID(m)
			mName := getName(m)

			payload := make(map[string]interface{})
			if mID != "" && mID != "-" {
				payload["id"] = mID
			} else if mName != "" && mName != "N/A" {
				payload["model_name"] = mName
			}

			var result map[string]interface{}
			err := apiClient.Request("POST", "/model/delete", payload, &result)
			if err != nil {
				fmt.Printf("Error deleting %s: %v\n", mName, err)
			} else {
				fmt.Printf("Deleted: %s\n", mName)
			}
		}
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&flagRmDryRun, "dry-run", "n", false, "Dry run: show what would be deleted without deleting")
	rmCmd.Flags().BoolVarP(&flagRmYes, "yes", "y", false, "Skip confirmation prompt")
	rmCmd.Flags().StringArrayVarP(&flagRmFilters, "filter", "f", nil, "Filter (e.g. -f 'provider:deepinfra')")
}
