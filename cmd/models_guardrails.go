package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/MiGoller/litellm-mux/internal/client"
	"github.com/MiGoller/litellm-mux/internal/models"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

var (
	flagGuardrailAdd    []string
	flagGuardrailRemove []string
	flagGrDryRun        bool
	flagGrYes           bool
)

// currentGuardrails extracts the guardrail list from litellm_params.
func currentGuardrails(m models.ModelData) []string {
	var guards []string
	switch t := m.LitellmParams["guardrails"].(type) {
	case []interface{}:
		for _, item := range t {
			guards = append(guards, fmt.Sprintf("%v", item))
		}
	case []string:
		guards = append(guards, t...)
	case string:
		if t != "" {
			guards = append(guards, t)
		}
	}
	return guards
}

var guardrailsModelCmd = &cobra.Command{
	Use:   "guardrails",
	Short: "Manage guardrail assignments of models",
}

var grLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Show assigned guardrails (same view as 'models ls --guardrails')",
	Run: func(cmd *cobra.Command, args []string) {
		flagGuardrails = true
		lsCmd.Run(lsCmd, args)
	},
}

var grAddCmd = &cobra.Command{
	Use:   "add [models...]",
	Short: "Assign one or more guardrails to models",
	Run: func(cmd *cobra.Command, args []string) {
		runGuardrailMutation(args, func(existing []string) []string {
			updated := append([]string{}, existing...)
			for _, g := range flagGuardrailAdd {
				found := false
				for _, e := range updated {
					if e == g {
						found = true
						break
					}
				}
				if !found {
					updated = append(updated, g)
				}
			}
			return updated
		})
	},
}

var grRmCmd = &cobra.Command{
	Use:   "rm [models...]",
	Short: "Unassign one or more guardrails from models",
	Run: func(cmd *cobra.Command, args []string) {
		runGuardrailMutation(args, func(existing []string) []string {
			var updated []string
			for _, e := range existing {
				remove := false
				for _, g := range flagGuardrailRemove {
					if e == g {
						remove = true
						break
					}
				}
				if !remove {
					updated = append(updated, e)
				}
			}
			return updated
		})
	},
}

func runGuardrailMutation(args []string, mutate func(existing []string) []string) {
	if len(flagGuardrailAdd) == 0 && len(flagGuardrailRemove) == 0 {
		fmt.Fprintln(os.Stderr, "Error: provide at least one guardrail via --guardrail.")
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
		Before    []string
		After     []string
		Payload   map[string]interface{}
	}
	var plans []plan

	for _, m := range selected {
		modelID, _ := m.ModelInfo["id"].(string)
		before := currentGuardrails(m)
		after := mutate(before)
		if after == nil {
			// Ensure an explicit empty array is sent instead of null,
			// otherwise the API keeps the existing values.
			after = []string{}
		}

		lpCopy := make(map[string]interface{})
		for k, v := range m.LitellmParams {
			lpCopy[k] = v
		}
		lpCopy["guardrails"] = after

		payload := map[string]interface{}{
			"model_info":     map[string]interface{}{"id": modelID},
			"litellm_params": lpCopy,
		}

		plans = append(plans, plan{
			ModelName: modelDisplayName(m),
			ModelID:   modelID,
			Before:    before,
			After:     after,
			Payload:   payload,
		})
	}

	fmt.Println("Planned guardrail updates:")
	fmt.Println(strings.Repeat("-", 70))
	for _, p := range plans {
		beforeStr := "-"
		if len(p.Before) > 0 {
			beforeStr = strings.Join(p.Before, ", ")
		}
		afterStr := "(none)"
		if len(p.After) > 0 {
			afterStr = strings.Join(p.After, ", ")
		}
		fmt.Printf("  %s (ID: %s)\n", p.ModelName, p.ModelID)
		fmt.Printf("    Guardrails before: %s\n", beforeStr)
		fmt.Printf("    Guardrails after:  %s\n", afterStr)
		fmt.Println("    ---")
	}
	fmt.Println(strings.Repeat("-", 70))

	if flagGrDryRun {
		fmt.Println("[Dry-Run] Nothing was changed.")
		return
	}

	if !flagGrYes {
		fmt.Printf("Do you want to apply these %d guardrail update(s)? [y/N]: ", len(plans))
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
			fmt.Printf("Error updating guardrails of %s: %v\n", p.ModelName, err)
		} else {
			fmt.Printf("Updated guardrails: %s\n", p.ModelName)
		}
	}
}

func init() {
	grAddCmd.Flags().StringArrayVarP(&flagGuardrailAdd, "guardrail", "g", nil, "Guardrail to assign (repeatable)")
	grRmCmd.Flags().StringArrayVarP(&flagGuardrailRemove, "guardrail", "g", nil, "Guardrail to unassign (repeatable)")
	// Accept --guardrails as an alias for --guardrail on both commands
	for _, c := range []*cobra.Command{grAddCmd, grRmCmd} {
		if f := c.Flags().Lookup("guardrail"); f != nil {
			grAlias := &flag.Flag{
				Name:     "guardrails",
				Usage:    "Alias for --guardrail (repeatable)",
				Value:    f.Value,
				DefValue: f.DefValue,
			}
			c.Flags().AddFlag(grAlias)
		}
	}
	grAddCmd.Flags().BoolVarP(&flagGrDryRun, "dry-run", "n", false, "Dry run: show the plan without changing anything")
	grAddCmd.Flags().BoolVarP(&flagGrYes, "yes", "y", false, "Skip confirmation prompt")
	grRmCmd.Flags().BoolVarP(&flagGrDryRun, "dry-run", "n", false, "Dry run: show the plan without changing anything")
	grRmCmd.Flags().BoolVarP(&flagGrYes, "yes", "y", false, "Skip confirmation prompt")

	guardrailsModelCmd.AddCommand(grLsCmd)
	grLsCmd.Flags().AddFlagSet(lsCmd.Flags())
	guardrailsModelCmd.AddCommand(grAddCmd)
	guardrailsModelCmd.AddCommand(grRmCmd)
	modelsCmd.AddCommand(guardrailsModelCmd)
}
