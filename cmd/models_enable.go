package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/MiGoller/litellm-mux/internal/client"
	"github.com/MiGoller/litellm-mux/internal/models"
	"github.com/spf13/cobra"
)

var (
	flagEnableDisable bool // true = enable (blocked=false), false = disable (blocked=true)
	flagDryRun        bool
	flagYes           bool
)

var enableCmd = &cobra.Command{
	Use:   "enable [models...]",
	Short: "Enable models (considering -f filters)",
	Run: func(cmd *cobra.Command, args []string) {
		flagEnableDisable = true
		enableDisableCmdRun(cmd, args)
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable [models...]",
	Short: "Disable models (considering -f filters)",
	Run: func(cmd *cobra.Command, args []string) {
		flagEnableDisable = false
		enableDisableCmdRun(cmd, args)
	},
}

func isModelCurrentlyBlocked(m models.ModelData) bool {
	for _, mp := range []map[string]interface{}{m.ModelInfo, m.LitellmParams} {
		if dl, ok := mp["blocked"]; ok {
			switch v := dl.(type) {
			case bool:
				return v
			case string:
				return v == "true" || v == "1"
			case float64:
				return v == 1
			}
		}
	}
	return false
}

type enableDisablePlan struct {
	ModelName string
	ModelID   string
	Before    bool // current blocked state
	After     bool // target blocked state
	Payload   map[string]interface{}
}

func runEnableDisable(args []string) ([]enableDisablePlan, error) {
	targetEnable := flagEnableDisable
	targetBlocked := !targetEnable

	cfg := GetConfig()
	apiClient := client.NewClient(cfg.LiteLLMURL, cfg.MasterKey)

	resp := fetchModels(apiClient)
	selected := selectModels(resp, args)
	if len(selected) == 0 {
		fmt.Println("No matching models found.")
		return nil, nil
	}

	var plans []enableDisablePlan

	for _, m := range selected {
		modelID, _ := m.ModelInfo["id"].(string)
		if modelID == "" {
			modelID, _ = m.ModelInfo["model_id"].(string)
		}
		mName := m.ModelName
		if mName == "" {
			if mn, ok := m.ModelInfo["model_name"].(string); ok {
				mName = mn
			}
		}

		currentBlocked := isModelCurrentlyBlocked(m)

		// Build payload matching tags/guardrails pattern (/model/update)
		lpCopy := make(map[string]interface{})
		for k, v := range m.LitellmParams {
			lpCopy[k] = v
		}
		lpCopy["blocked"] = targetBlocked

		miCopy := make(map[string]interface{})
		for k, v := range m.ModelInfo {
			if k != "id" && k != "db_model" && k != "key" {
				miCopy[k] = v
			}
		}
		miCopy["blocked"] = targetBlocked

		payload := map[string]interface{}{
			"model_info":     miCopy,
			"litellm_params": lpCopy,
		}
		if modelID != "" && modelID != "-" {
			// Ensure model_info has the ID for the update API
			if miMap, ok := payload["model_info"].(map[string]interface{}); ok {
				miMap["id"] = modelID
			}
		}

		plans = append(plans, enableDisablePlan{
			ModelName: enableModelDisplayName(m),
			ModelID:   modelID,
			Before:    currentBlocked,
			After:     targetBlocked,
			Payload:   payload,
		})
	}

	return plans, nil
}

func enableModelDisplayName(m models.ModelData) string {
	if m.ModelName != "" {
		return m.ModelName
	}
	if mn, ok := m.ModelInfo["model_name"].(string); ok {
		return mn
	}
	return "N/A"
}

func printEnableDisablePlan(plans []enableDisablePlan, enable bool) {
	action := "enable"
	if !enable {
		action = "disable"
	}
	fmt.Printf("Planned %s model updates:\n", action)
	fmt.Println(strings.Repeat("-", 70))
	for _, p := range plans {
		beforeStr := "blocked"
		if !p.Before {
			beforeStr = "active"
		}
		afterStr := "blocked"
		if !p.After {
			afterStr = "active"
		}
		fmt.Printf("  %s (ID: %s)\n", p.ModelName, p.ModelID)
		fmt.Printf("    Before: %s\n", beforeStr)
		fmt.Printf("    After:  %s\n", afterStr)
		fmt.Println("    ---")
	}
	fmt.Println(strings.Repeat("-", 70))
}

func enableDisableCmdRun(cmd *cobra.Command, args []string) {
	plans, err := runEnableDisable(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error preparing plans: %v\n", err)
		os.Exit(1)
	}

	if len(plans) == 0 {
		return
	}

	printEnableDisablePlan(plans, flagEnableDisable)

	if flagDryRun {
		fmt.Println("[Dry-Run] Nothing was changed.")
		return
	}

	if !flagYes {
		toggle := "enable"
		if !flagEnableDisable {
			toggle = "disable"
		}
		fmt.Printf("Do you want to %s these %d model(s)? [y/N]: ", toggle, len(plans))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Aborted.")
			return
		}
	}

	cfg := GetConfig()
	apiClient := client.NewClient(cfg.LiteLLMURL, cfg.MasterKey)

	actionStr := "enable"
	if !flagEnableDisable {
		actionStr = "disable"
	}

	for _, p := range plans {
		var result map[string]interface{}
		err := apiClient.Request("POST", "/model/update", p.Payload, &result)
		if err != nil {
			fmt.Printf("Error %s model %s: %v\n", actionStr, p.ModelName, err)
		} else {
			fmt.Printf("Successfully %s: %s\n", actionStr, p.ModelName)
		}
	}
}

func init() {
	for _, c := range []*cobra.Command{enableCmd, disableCmd} {
		c.Flags().BoolVarP(&flagDryRun, "dry-run", "n", false, "Show the plan without making changes")
		c.Flags().BoolVarP(&flagYes, "yes", "y", false, "Skip confirmation prompt")
		modelsCmd.AddCommand(c)
	}
}
