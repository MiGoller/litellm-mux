package cmd

import (
	"fmt"
	"os"

	"github.com/MiGoller/litellm-mux/internal/client"
	"github.com/MiGoller/litellm-mux/internal/filter"
	"github.com/MiGoller/litellm-mux/internal/models"
	"github.com/spf13/cobra"
)

// flagModelsFilters is the persistent filter flag on the `models` command group.
var flagModelsFilters []string

// fullHeaders are the headers of the widest possible `models ls -a` view.
// Filters are always evaluated against this full row so that every column
// is filterable regardless of the display options of the sub-command.
var fullHeaders = []string{
	"MODEL NAME", "ID", "PROVIDER", "PROVIDER MODEL", "TAGS", "GUARDRAILS", "STATUS",
	"MAX TOKENS", "INPUT / 1M ($)", "OUTPUT / 1M ($)", "MODE", "API BASE", "CREDENTIAL",
}

// modelToFullRow converts a model into the full row representation used for filtering.
func modelToFullRow(m models.ModelData) []string {
	modelInfo := m.ModelInfo
	litellmParams := m.LitellmParams

	modelName := m.ModelName
	if modelName == "" {
		if mn, ok := modelInfo["model_name"].(string); ok {
			modelName = mn
		} else {
			modelName = "N/A"
		}
	}

	modelID, _ := modelInfo["id"].(string)
	if modelID == "" {
		modelID, _ = modelInfo["model_id"].(string)
	}
	if modelID == "" {
		modelID = "-"
	}

	provider, _ := litellmParams["custom_llm_provider"].(string)
	if provider == "" {
		provider, _ = litellmParams["model"].(string)
	}
	if provider == "" {
		provider = "N/A"
	}

	providerModel, _ := litellmParams["model"].(string)
	if providerModel == "" {
		providerModel = "-"
	}

	maxTokens := "-"
	if mt, ok := modelInfo["max_tokens"]; ok && mt != nil {
		maxTokens = fmt.Sprintf("%v", mt)
	} else if mt, ok := litellmParams["max_tokens"]; ok && mt != nil {
		maxTokens = fmt.Sprintf("%v", mt)
	}

	inputStr := "-"
	outputStr := "-"
	if ic, ok := modelInfo["input_cost_per_token"].(float64); ok {
		inputStr = fmt.Sprintf("%.2f", ic*1_000_000)
	}
	if oc, ok := modelInfo["output_cost_per_token"].(float64); ok {
		outputStr = fmt.Sprintf("%.2f", oc*1_000_000)
	}

	mode := "-"
	if md, ok := modelInfo["mode"].(string); ok {
		mode = md
	} else if md, ok := litellmParams["mode"].(string); ok {
		mode = md
	}

	apiBase := "-"
	if ab, ok := litellmParams["api_base"].(string); ok {
		apiBase = ab
	}

	cred := "-"
	if c, ok := modelInfo["litellm_credential_name"].(string); ok {
		cred = c
	} else if c, ok := litellmParams["litellm_credential_name"].(string); ok {
		cred = c
	}

	statusStr := "active"
	if dl, ok := litellmParams["disabled"]; ok {
		disabled := false
		switch v := dl.(type) {
		case bool:
			disabled = v
		case string:
			disabled = (v == "true" || v == "1")
		case float64:
			disabled = (v == 1)
		}
		if disabled {
			statusStr = "disabled"
		}
	}

	return []string{
		modelName, modelID, provider, providerModel, formatTags(litellmParams["tags"]), formatTags(litellmParams["guardrails"]), statusStr,
		maxTokens, inputStr, outputStr, mode, apiBase, cred,
	}
}

// fetchModels loads all models from the gateway.
func fetchModels(apiClient *client.Client) models.ModelInfoResponse {
	var resp models.ModelInfoResponse
	if err := apiClient.Request("GET", "/model/info", nil, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching models: %v\n", err)
		os.Exit(1)
	}
	return resp
}

// selectModels resolves the target model set: direct name/ID args plus the
// persistent `-f` filter, evaluated against the full row representation.
func selectModels(resp models.ModelInfoResponse, args []string) []models.ModelData {
	var selected []models.ModelData
	seen := make(map[string]bool)

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
		return id
	}

	addSelected := func(m models.ModelData) {
		key := getID(m)
		if key == "" {
			key = getName(m)
		}
		if !seen[key] {
			selected = append(selected, m)
			seen[key] = true
		}
	}

	// 1. Direct model names/IDs from positional args
	for _, target := range args {
		found := false
		for _, m := range resp.Data {
			if target == getName(m) || target == getID(m) {
				addSelected(m)
				found = true
			}
		}
		if !found {
			fmt.Printf("Warning: model '%s' was not found on the server.\n", target)
		}
	}

	// 2. Persistent filter (-f), evaluated against the full row
	if len(flagModelsFilters) > 0 {
		compiledFilters, rawFilters := filter.ParseFilters(flagModelsFilters, fullHeaders)
		if len(rawFilters) > 0 {
			for _, rf := range rawFilters {
				fmt.Fprintf(os.Stderr, "Error: invalid filter (bad regex): %s\n", rf)
			}
			os.Exit(1)
		}
		for _, m := range resp.Data {
			row := modelToFullRow(m)
			if filter.EvaluateRow(row, compiledFilters) {
				addSelected(m)
			}
		}
	}

	return selected
}

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage LiteLLM proxy models",
}

func init() {
	modelsCmd.PersistentFlags().StringArrayVarP(&flagModelsFilters, "filter", "f", nil, "Filter to select models (e.g. -f 'provider:deepinfra', repeatable, AND-combined)")

	rootCmd.AddCommand(modelsCmd)
	modelsCmd.AddCommand(lsCmd)
	modelsCmd.AddCommand(rmCmd)
	modelsCmd.AddCommand(copyCmd)
	modelsCmd.AddCommand(tagCmd)
}
