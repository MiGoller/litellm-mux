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
	flagTagAdd    []string
	flagTagRemove []string
	flagTagDryRun bool
	flagTagYes    bool
)

func currentTags(m models.ModelData) []string {
	var tags []string
	switch t := m.LitellmParams["tags"].(type) {
	case []interface{}:
		for _, item := range t {
			tags = append(tags, fmt.Sprintf("%v", item))
		}
	case []string:
		tags = append(tags, t...)
	case string:
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

var tagCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags of models",
}

var tagLsCmd = &cobra.Command{
	Use:   "ls [filters...]",
	Short: "Show model tags (same view as 'models ls --tags')",
	Run: func(cmd *cobra.Command, args []string) {
		// Force the TAGS column and delegate to the regular `models ls` implementation.
		flagTags = true
		lsCmd.Run(lsCmd, args)
	},
}

var tagAddCmd = &cobra.Command{
	Use:   "add [models...]",
	Short: "Add one or more tags to models",
	Run: func(cmd *cobra.Command, args []string) {
		runTagMutation(args, func(existing []string) []string {
			updated := append([]string{}, existing...)
			for _, t := range flagTagAdd {
				found := false
				for _, e := range updated {
					if e == t {
						found = true
						break
					}
				}
				if !found {
					updated = append(updated, t)
				}
			}
			return updated
		})
	},
}

var tagRmCmd = &cobra.Command{
	Use:   "rm [models...]",
	Short: "Remove one or more tags from models",
	Run: func(cmd *cobra.Command, args []string) {
		runTagMutation(args, func(existing []string) []string {
			var updated []string
			for _, e := range existing {
				remove := false
				for _, t := range flagTagRemove {
					if e == t {
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

func modelDisplayName(m models.ModelData) string {
	if m.ModelName != "" {
		return m.ModelName
	}
	if mn, ok := m.ModelInfo["model_name"].(string); ok {
		return mn
	}
	return "N/A"
}

func runTagMutation(args []string, mutate func(existing []string) []string) {
	if len(flagTagAdd) == 0 && len(flagTagRemove) == 0 {
		fmt.Fprintln(os.Stderr, "Error: provide at least one tag via --tag.")
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
		before := currentTags(m)
		after := mutate(before)
		if after == nil {
			// Ensure an explicit empty array is sent instead of null,
			// otherwise the API keeps the existing tags.
			after = []string{}
		}

		lpCopy := make(map[string]interface{})
		for k, v := range m.LitellmParams {
			lpCopy[k] = v
		}
		lpCopy["tags"] = after

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

	fmt.Println("Planned tag updates:")
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
		fmt.Printf("    Tags before: %s\n", beforeStr)
		fmt.Printf("    Tags after:  %s\n", afterStr)
		fmt.Println("    ---")
	}
	fmt.Println(strings.Repeat("-", 70))

	if flagTagDryRun {
		fmt.Println("[Dry-Run] Nothing was changed.")
		return
	}

	if !flagTagYes {
		fmt.Printf("Do you want to apply these %d tag update(s)? [y/N]: ", len(plans))
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
			fmt.Printf("Error updating tags of %s: %v\n", p.ModelName, err)
		} else {
			fmt.Printf("Updated tags: %s\n", p.ModelName)
		}
	}
}

func init() {
	tagLsCmd.Flags().AddFlagSet(lsCmd.Flags())
	tagAddCmd.Flags().StringArrayVarP(&flagTagAdd, "tag", "t", nil, "Tag to add (repeatable)")
	tagRmCmd.Flags().StringArrayVarP(&flagTagRemove, "tag", "t", nil, "Tag to remove (repeatable)")
	// Accept --tags as an alias for --tag on both commands
	for _, c := range []*cobra.Command{tagAddCmd, tagRmCmd} {
		if f := c.Flags().Lookup("tag"); f != nil {
			tagsAlias := &flag.Flag{
				Name:     "tags",
				Usage:    "Alias for --tag (repeatable)",
				Value:    f.Value,
				DefValue: f.DefValue,
			}
			c.Flags().AddFlag(tagsAlias)
		}
	}
	tagAddCmd.Flags().BoolVarP(&flagTagDryRun, "dry-run", "n", false, "Dry run: show the plan without changing anything")
	tagAddCmd.Flags().BoolVarP(&flagTagYes, "yes", "y", false, "Skip confirmation prompt")
	tagRmCmd.Flags().BoolVarP(&flagTagDryRun, "dry-run", "n", false, "Dry run: show the plan without changing anything")
	tagRmCmd.Flags().BoolVarP(&flagTagYes, "yes", "y", false, "Skip confirmation prompt")

	tagCmd.AddCommand(tagLsCmd)
	tagCmd.AddCommand(tagAddCmd)
	tagCmd.AddCommand(tagRmCmd)
}
