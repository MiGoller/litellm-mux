package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/MiGoller/litellm-mux/internal/client"
	"github.com/MiGoller/litellm-mux/internal/filter"
	"github.com/spf13/cobra"
)

var (
	flagMinimal    bool
	flagTokens     bool
	flagCosts      bool
	flagMode       bool
	flagApiBase    bool
	flagCredential bool
	flagModelStr   bool
	flagModelID    bool
	flagTags       bool
	flagGuardrails bool
	flagStatus     bool
	flagAll        bool
	flagOneline    bool
)

var lsCmd = &cobra.Command{
	Use:   "ls [filters...]",
	Short: "List all available models",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetConfig()
		apiClient := client.NewClient(cfg.LiteLLMURL, cfg.MasterKey)

		resp := fetchModels(apiClient)
		if len(resp.Data) == 0 {
			fmt.Println("No models found.")
			return
		}

		showTokens := flagAll || flagTokens
		showCosts := flagAll || flagCosts
		showMode := flagAll || flagMode
		showApiBase := flagAll || flagApiBase
		showCredential := flagAll || flagCredential
		showModelStr := flagAll || flagModelStr
		showModelID := flagAll || flagModelID
		showTags := flagAll || flagTags
		showGuardrails := flagAll || flagGuardrails
		showStatus := flagAll || flagStatus

		if !flagMinimal && !flagTokens && !flagCosts && !flagMode && !flagApiBase && !flagCredential && !flagModelStr && !flagModelID && !flagTags && !flagGuardrails && !flagStatus && !flagAll && !flagOneline {
			showTokens = true
			showCosts = true
		}

		var headers []string
		var alignments []string

		headers = append(headers, "MODEL NAME")
		alignments = append(alignments, "<")

		if showModelID {
			headers = append(headers, "ID")
			alignments = append(alignments, "<")
		}

		headers = append(headers, "PROVIDER")
		alignments = append(alignments, "<")

		if showModelStr {
			headers = append(headers, "PROVIDER MODEL")
			alignments = append(alignments, "<")
		}
		if showTags {
			headers = append(headers, "TAGS")
			alignments = append(alignments, "<")
		}
		if showGuardrails {
			headers = append(headers, "GUARDRAILS")
			alignments = append(alignments, "<")
		}
		if showStatus {
			headers = append(headers, "STATUS")
			alignments = append(alignments, "<")
		}
		if showTokens {
			headers = append(headers, "MAX TOKENS")
			alignments = append(alignments, ">")
		}
		if showCosts {
			headers = append(headers, "INPUT / 1M ($)", "OUTPUT / 1M ($)")
			alignments = append(alignments, ">", ">")
		}
		if showMode {
			headers = append(headers, "MODE")
			alignments = append(alignments, "<")
		}
		if showApiBase {
			headers = append(headers, "API BASE")
			alignments = append(alignments, "<")
		}
		if showCredential {
			headers = append(headers, "CREDENTIAL")
			alignments = append(alignments, "<")
		}

		// Persistent -f filters (set on the `models` group) select the model
		// set; positional args remain display-level regex filters on the
		// shown columns.
		selectedModels := resp.Data
		if len(flagModelsFilters) > 0 {
			selectedModels = selectModels(resp, nil)
		}

		var rows [][]string
		for _, m := range selectedModels {
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

			tagsStr := formatTags(litellmParams["tags"])

			row := []string{modelName}
			if showModelID {
				row = append(row, modelID)
			}
			row = append(row, provider)

			if showModelStr {
				row = append(row, providerModel)
			}
			if showTags {
				row = append(row, tagsStr)
			}
			if showGuardrails {
				row = append(row, formatTags(litellmParams["guardrails"]))
			}
			if showStatus {
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
				row = append(row, statusStr)
			}

			if showTokens {
				maxTokens := "-"
				if mt, ok := modelInfo["max_tokens"]; ok && mt != nil {
					maxTokens = fmt.Sprintf("%v", mt)
				} else if mt, ok := litellmParams["max_tokens"]; ok && mt != nil {
					maxTokens = fmt.Sprintf("%v", mt)
				}
				row = append(row, maxTokens)
			}

			if showCosts {
				inputStr := "-"
				outputStr := "-"
				if ic, ok := modelInfo["input_cost_per_token"].(float64); ok {
					inputStr = fmt.Sprintf("%.2f", ic*1_000_000)
				}
				if oc, ok := modelInfo["output_cost_per_token"].(float64); ok {
					outputStr = fmt.Sprintf("%.2f", oc*1_000_000)
				}
				row = append(row, inputStr, outputStr)
			}

			if showMode {
				mode := "-"
				if md, ok := modelInfo["mode"].(string); ok {
					mode = md
				} else if md, ok := litellmParams["mode"].(string); ok {
					mode = md
				}
				row = append(row, mode)
			}

			if showApiBase {
				apiBase := "-"
				if ab, ok := litellmParams["api_base"].(string); ok {
					apiBase = ab
				}
				row = append(row, apiBase)
			}

			if showCredential {
				cred := "-"
				if c, ok := modelInfo["litellm_credential_name"].(string); ok {
					cred = c
				} else if c, ok := litellmParams["litellm_credential_name"].(string); ok {
					cred = c
				}
				row = append(row, cred)
			}

			rows = append(rows, row)
		}

		compiledFilters, rawFilters := filter.ParseFilters(args, headers)
		if len(rawFilters) > 0 {
			for _, rf := range rawFilters {
				fmt.Fprintf(os.Stderr, "Error: invalid filter (bad regex): %s\n", rf)
			}
			os.Exit(1)
		}

		var filteredRows [][]string
		for _, row := range rows {
			if filter.EvaluateRow(row, compiledFilters) {
				filteredRows = append(filteredRows, row)
			}
		}

		if len(filteredRows) == 0 {
			fmt.Println("No models match the filter.")
			return
		}

		if flagOneline {
			var names []string
			for _, r := range filteredRows {
				names = append(names, r[0])
			}
			fmt.Println(strings.Join(names, " "))
			return
		}

		// Calculate column widths
		colWidths := make([]int, len(headers))
		for i, h := range headers {
			colWidths[i] = len(h)
		}
		for _, r := range filteredRows {
			for i, val := range r {
				if len(val) > colWidths[i] {
					colWidths[i] = len(val)
				}
			}
		}

		// Format output
		var formatParts []string
		for i, align := range alignments {
			formatParts = append(formatParts, fmt.Sprintf("%%-%ds", colWidths[i]))
			if align == ">" {
				formatParts[i] = fmt.Sprintf("%%%ds", colWidths[i])
			}
		}
		rowFormat := strings.Join(formatParts, " | ")

		// Print header
		headerArgs := make([]interface{}, len(headers))
		for i, h := range headers {
			headerArgs[i] = h
		}
		fmt.Printf(rowFormat+"\n", headerArgs...)

		// Print separator
		totalWidth := 0
		for _, w := range colWidths {
			totalWidth += w
		}
		totalWidth += (len(headers) - 1) * 3
		fmt.Println(strings.Repeat("-", totalWidth))

		// Print rows
		for _, r := range filteredRows {
			rowArgs := make([]interface{}, len(r))
			for i, val := range r {
				rowArgs[i] = val
			}
			fmt.Printf(rowFormat+"\n", rowArgs...)
		}
	},
}

func formatTags(val interface{}) string {
	switch t := val.(type) {
	case string:
		return t
	case []string:
		return strings.Join(t, ", ")
	case []interface{}:
		var parts []string
		for _, item := range t {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		if len(parts) == 0 {
			return "-"
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", t)
	}
}

func init() {
	lsCmd.Flags().BoolVarP(&flagMinimal, "minimal", "l", false, "Minimal view (model name and provider only)")
	lsCmd.Flags().BoolVar(&flagTokens, "tokens", false, "Show max tokens")
	lsCmd.Flags().BoolVar(&flagCosts, "costs", false, "Show costs")
	lsCmd.Flags().BoolVar(&flagMode, "mode", false, "Show mode")
	lsCmd.Flags().BoolVar(&flagApiBase, "api-base", false, "Show API base")
	lsCmd.Flags().BoolVar(&flagCredential, "credential", false, "Show credential")
	lsCmd.Flags().BoolVar(&flagModelStr, "model-string", false, "Show the model string at the provider")
	lsCmd.Flags().BoolVar(&flagModelID, "id", false, "Show the LiteLLM model ID")
	lsCmd.Flags().BoolVar(&flagTags, "tags", false, "Show model tags")
	lsCmd.Flags().BoolVar(&flagGuardrails, "guardrails", false, "Show assigned guardrails")
	lsCmd.Flags().BoolVar(&flagStatus, "status", false, "Show model status (active/disabled)")
	lsCmd.Flags().BoolVarP(&flagAll, "all", "a", false, "Show all columns")
	lsCmd.Flags().BoolVarP(&flagOneline, "oneline", "1", false, "Model names only, on a single line")
}
