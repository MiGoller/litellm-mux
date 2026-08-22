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
	flagRmDryRun bool
	flagRmYes    bool
)

var rmCmd = &cobra.Command{
	Use:   "rm [models...]",
	Short: "Delete one or more models (by name/ID or -f filter)",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 && len(flagModelsFilters) == 0 {
			fmt.Fprintln(os.Stderr, "Error: provide model names or filters (-f).")
			os.Exit(1)
		}

		cfg := GetConfig()
		apiClient := client.NewClient(cfg.LiteLLMURL, cfg.MasterKey)

		resp := fetchModels(apiClient)
		modelsToDelete := selectModels(resp, args)

		if len(modelsToDelete) == 0 {
			fmt.Println("No matching models to delete.")
			return
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
}
