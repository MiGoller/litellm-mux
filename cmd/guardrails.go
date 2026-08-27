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
	flagGuardrailLsJSON bool
	flagGrFilters       []string
)

// guardrailInfo mirrors the entries returned by GET /guardrails/list.
type guardrailListResponse struct {
	Guardrails []map[string]interface{} `json:"guardrails"`
}

var guardrailsCmd = &cobra.Command{
	Use:   "guardrails",
	Short: "Inspect guardrails defined on the LiteLLM gateway",
}

var guardrailsLsCmd = &cobra.Command{
	Use:   "ls [filters...]",
	Short: "List guardrails defined on the gateway (with mode)",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := GetConfig()
		apiClient := client.NewClient(cfg.LiteLLMURL, cfg.MasterKey)

		var resp guardrailListResponse
		if err := apiClient.Request("GET", "/guardrails/list", nil, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching guardrails: %v\n", err)
			os.Exit(1)
		}

		// /guardrails/list only returns config.yaml guardrails; DB-defined
		// ones (added via the UI) live behind /v2/guardrails/list. Merge both.
		var respV2 guardrailListResponse
		if err := apiClient.Request("GET", "/v2/guardrails/list", nil, &respV2); err == nil {
			resp.Guardrails = append(resp.Guardrails, respV2.Guardrails...)
		}

		// Deduplicate by guardrail name (a guardrail may exist in both).
		seen := make(map[string]bool)
		var unique []map[string]interface{}
		for _, g := range resp.Guardrails {
			name := fmt.Sprintf("%v", g["guardrail_name"])
			if !seen[name] {
				seen[name] = true
				unique = append(unique, g)
			}
		}
		resp.Guardrails = unique

		if flagGuardrailLsJSON {
			for _, g := range resp.Guardrails {
				fmt.Printf("%v\n", g)
			}
			return
		}

		if len(resp.Guardrails) == 0 {
			fmt.Println("No guardrails defined on the gateway.")
			return
		}

		headers := []string{"GUARDRAIL NAME", "GUARDRAIL", "MODE"}
		rows := make([][]string, 0, len(resp.Guardrails))
		for _, g := range resp.Guardrails {
			name, _ := g["guardrail_name"].(string)
			grType, _ := g["litellm_params"].(map[string]interface{})
			guard := ""
			mode := ""
			if grType != nil {
				guard, _ = grType["guardrail"].(string)
				mode = formatGuardrailMode(grType["mode"])
			}
			rows = append(rows, []string{name, guard, mode})
		}

		// Persistent -f filters (set on the `guardrails` group) select the
		// guardrail set; positional args are additional display-level regex
		// filters on the shown columns.
		allRows := rows
		if len(flagGrFilters) > 0 {
			groupFilters, groupRaw := filter.ParseFilters(flagGrFilters, headers)
			if len(groupRaw) > 0 {
				for _, rf := range groupRaw {
					fmt.Fprintf(os.Stderr, "Error: invalid filter (bad regex): %s\n", rf)
				}
				os.Exit(1)
			}
			var selected [][]string
			for _, r := range allRows {
				if filter.EvaluateRow(r, groupFilters) {
					selected = append(selected, r)
				}
			}
			rows = selected
		}

		// Reuse the shared filter engine: positional args are regex filters
		// matched against all columns or column-scoped via prefix
		// (e.g. "guardrailname:pii", "mode:pre_call", "guardrail:presidio").
		compiledFilters, rawFilters := filter.ParseFilters(args, headers)
		if len(rawFilters) > 0 {
			for _, rf := range rawFilters {
				fmt.Fprintf(os.Stderr, "Error: invalid filter (bad regex): %s\n", rf)
			}
			os.Exit(1)
		}

		var filteredRows [][]string
		for _, r := range rows {
			if filter.EvaluateRow(r, compiledFilters) {
				filteredRows = append(filteredRows, r)
			}
		}

		if len(filteredRows) == 0 {
			fmt.Println("No guardrails match the filter.")
			return
		}

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

		formatParts := make([]string, len(headers))
		for i := range headers {
			formatParts[i] = fmt.Sprintf("%%-%ds", colWidths[i])
		}
		rowFormat := strings.Join(formatParts, " | ")

		headerArgs := make([]interface{}, len(headers))
		for i, h := range headers {
			headerArgs[i] = h
		}
		fmt.Printf(rowFormat+"\n", headerArgs...)

		totalWidth := 0
		for _, w := range colWidths {
			totalWidth += w
		}
		totalWidth += (len(headers) - 1) * 3
		fmt.Println(strings.Repeat("-", totalWidth))

		for _, r := range filteredRows {
			rowArgs := make([]interface{}, len(r))
			for i, val := range r {
				rowArgs[i] = val
			}
			fmt.Printf(rowFormat+"\n", rowArgs...)
		}
	},
}

// formatGuardrailMode renders the mode field, which can be a string or a list.
func formatGuardrailMode(v interface{}) string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return "-"
		}
		return t
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
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%v", v)
	}
}

func init() {
	guardrailsLsCmd.Flags().BoolVar(&flagGuardrailLsJSON, "json", false, "Print raw guardrail entries")
	guardrailsCmd.PersistentFlags().StringArrayVarP(&flagGrFilters, "filter", "f", nil, "Filter to select guardrails (e.g. -f 'mode:pre_call', repeatable, AND-combined)")

	guardrailsCmd.AddCommand(guardrailsLsCmd)
	rootCmd.AddCommand(guardrailsCmd)
}
