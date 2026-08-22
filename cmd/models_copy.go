package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MiGoller/litellm-mux/internal/client"
	"github.com/MiGoller/litellm-mux/internal/models"
	"github.com/spf13/cobra"
)

var (
	flagCopyProvider    string
	flagCopyCredential  string
	flagCopyAllCreds    bool
	flagCopyModelString string
	flagCopyDryRun      bool
	flagCopyYes         bool
)

var copyCmd = &cobra.Command{
	Use:   "copy [source_model] [new_model_name]",
	Short: "Copy / multiplex an existing model to a new provider or credential",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "Error: provide a source model.")
			os.Exit(1)
		}

		sourceTarget := args[0]
		var newModelNameArg string
		if len(args) > 1 {
			newModelNameArg = args[1]
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
			os.Exit(1)
		}

		var sourceModelObj *models.ModelData
		for _, m := range resp.Data {
			mName := m.ModelName
			if mName == "" {
				if mn, ok := m.ModelInfo["model_name"].(string); ok {
					mName = mn
				}
			}
			mID, _ := m.ModelInfo["id"].(string)
			if mID == "" {
				mID, _ = m.ModelInfo["model_id"].(string)
			}

			if sourceTarget == mName || sourceTarget == mID {
				sourceModelObj = &m
				break
			}
		}

		if sourceModelObj == nil {
			fmt.Printf("Error: source model '%s' was not found on the server.\n", sourceTarget)
			os.Exit(1)
		}

		srcName := sourceModelObj.ModelName
		if srcName == "" {
			if mn, ok := sourceModelObj.ModelInfo["model_name"].(string); ok {
				srcName = mn
			} else {
				srcName = "N/A"
			}
		}

		srcProvider, _ := sourceModelObj.LitellmParams["custom_llm_provider"].(string)
		if srcProvider == "" {
			srcProvider, _ = sourceModelObj.LitellmParams["model"].(string)
		}
		if srcProvider == "" {
			srcProvider = "N/A"
		}

		srcCred, _ := sourceModelObj.ModelInfo["litellm_credential_name"].(string)
		if srcCred == "" {
			srcCred, _ = sourceModelObj.LitellmParams["litellm_credential_name"].(string)
		}
		if srcCred == "" {
			srcCred = "N/A"
		}

		srcModelStr, _ := sourceModelObj.LitellmParams["model"].(string)
		if srcModelStr == "" {
			srcModelStr = "N/A"
		}

		targetProvider := flagCopyProvider
		if targetProvider == "" {
			targetProvider = srcProvider
		}

		targetCredential := flagCopyCredential
		if targetCredential == "" {
			targetCredential = srcCred
		}

		var targetCredentialsList []string
		if flagCopyAllCreds {
			seenCreds := make(map[string]bool)
			for _, m := range resp.Data {
				p, _ := m.LitellmParams["custom_llm_provider"].(string)
				if p == "" {
					p, _ = m.LitellmParams["model"].(string)
				}
				c, _ := m.ModelInfo["litellm_credential_name"].(string)
				if c == "" {
					c, _ = m.LitellmParams["litellm_credential_name"].(string)
				}

				if p == targetProvider && c != "" && c != srcCred {
					seenCreds[c] = true
				}
			}
			for c := range seenCreds {
				targetCredentialsList = append(targetCredentialsList, c)
			}
			sort.Strings(targetCredentialsList)
			if len(targetCredentialsList) == 0 {
				fmt.Printf("No other credentials found for provider '%s' (besides '%s') on the server.\n", targetProvider, srcCred)
				return
			}
		} else {
			targetCredentialsList = []string{targetCredential}
		}

		type CopyInfo struct {
			Name        string
			Provider    string
			Credential  string
			ModelString string
			Payload     map[string]interface{}
		}

		var copiesToCreate []CopyInfo

		for _, cred := range targetCredentialsList {
			var newName string
			if newModelNameArg != "" {
				if len(targetCredentialsList) > 1 {
					parts := strings.Split(cred, " - ")
					credSuffix := strings.ToLower(parts[len(parts)-1])
					credSuffix = strings.NewReplacer(" ", "-", "@", "-at-", ".", "-").Replace(credSuffix)
					newName = fmt.Sprintf("%s-%s", newModelNameArg, credSuffix)
				} else {
					newName = newModelNameArg
				}
			} else {
				if len(targetCredentialsList) > 1 {
					parts := strings.Split(cred, " - ")
					credSuffix := strings.ToLower(parts[len(parts)-1])
					credSuffix = strings.NewReplacer(" ", "-", "@", "-at-", ".", "-").Replace(credSuffix)
					newName = fmt.Sprintf("%s-%s", srcName, credSuffix)
				} else {
					newName = srcName
				}
			}

			payload := make(map[string]interface{})
			payload["model_name"] = newName

			miCopy := make(map[string]interface{})
			for k, v := range sourceModelObj.ModelInfo {
				if k != "id" && k != "db_model" && k != "key" {
					miCopy[k] = v
				}
			}
			payload["model_info"] = miCopy

			lpCopy := make(map[string]interface{})
			for k, v := range sourceModelObj.LitellmParams {
				lpCopy[k] = v
			}
			lpCopy["custom_llm_provider"] = targetProvider
			lpCopy["litellm_credential_name"] = cred
			if flagCopyModelString != "" {
				lpCopy["model"] = flagCopyModelString
			}
			payload["litellm_params"] = lpCopy

			mStr, _ := lpCopy["model"].(string)
			if mStr == "" {
				mStr = "N/A"
			}

			copiesToCreate = append(copiesToCreate, CopyInfo{
				Name:        newName,
				Provider:    targetProvider,
				Credential:  cred,
				ModelString: mStr,
				Payload:     payload,
			})
		}

		fmt.Printf("Planned model copies (multiplexing) [%d]:\n", len(copiesToCreate))
		fmt.Println(strings.Repeat("-", 70))
		fmt.Printf("  Source model:   %s (provider: %s, credential: %s)\n", srcName, srcProvider, srcCred)
		for _, c := range copiesToCreate {
			fmt.Printf("  -> New model:    %s\n", c.Name)
			fmt.Printf("     Provider:     %s\n", c.Provider)
			fmt.Printf("     Credential:   %s\n", c.Credential)
			fmt.Printf("     Model-String: %s\n", c.ModelString)
			fmt.Println("     ---")
		}
		fmt.Println(strings.Repeat("-", 70))

		if flagCopyDryRun {
			fmt.Println("[Dry-Run] Nothing was created.")
			return
		}

		if !flagCopyYes {
			fmt.Printf("Do you want to create these %d model(s)? [y/N]: ", len(copiesToCreate))
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input != "y" && input != "yes" {
				fmt.Println("Aborted.")
				return
			}
		}

		for _, c := range copiesToCreate {
			var result map[string]interface{}
			err := apiClient.Request("POST", "/model/new", c.Payload, &result)
			if err != nil {
				fmt.Printf("Error creating %s: %v\n", c.Name, err)
			} else {
				fmt.Printf("Successfully created: %s (credential: %s)\n", c.Name, c.Credential)
			}
		}
	},
}

func init() {
	copyCmd.Flags().StringVar(&flagCopyProvider, "provider", "", "Target provider")
	copyCmd.Flags().StringVar(&flagCopyCredential, "credential", "", "Credential name")
	copyCmd.Flags().BoolVar(&flagCopyAllCreds, "all-other-credentials", false, "Automatically create copies for all other credentials of the target provider")
	copyCmd.Flags().StringVar(&flagCopyModelString, "model-string", "", "Optional target model string")
	copyCmd.Flags().BoolVarP(&flagCopyDryRun, "dry-run", "n", false, "Dry run: show the plan without creating anything")
	copyCmd.Flags().BoolVarP(&flagCopyYes, "yes", "y", false, "Skip confirmation prompt")
}
