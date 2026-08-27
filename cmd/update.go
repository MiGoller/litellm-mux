package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"github.com/spf13/cobra"
)

const repoSlug = "MiGoller/litellm-mux"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update litellm-mux to the latest version from GitHub",
	Run: func(cmd *cobra.Command, args []string) {
		if version == "dev" {
			fmt.Println("Running development build. Cannot self-update.")
			return
		}

		fmt.Printf("Checking for updates (current: %s)...\n", version)

		latest, found, err := selfupdate.DetectLatest(context.Background(), selfupdate.ParseSlug(repoSlug))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error occurred while detecting version: %v\n", err)
			return
		}
		if !found {
			fmt.Printf("Latest version for %s could not be found from github repository\n", repoSlug)
			return
		}

		if latest.LessOrEqual(version) {
			fmt.Printf("Current version (%s) is the latest or newer than %s\n", version, latest.Version())
			return
		}

		fmt.Printf("Updating to version %s...\n", latest.Version())
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not locate executable path: %v\n", err)
			return
		}

		if err := selfupdate.UpdateTo(context.Background(), latest.AssetURL, latest.AssetName, executable); err != nil {
			fmt.Fprintf(os.Stderr, "Error occurred while updating binary: %v\n", err)
			return
		}

		fmt.Printf("Successfully updated to version %s\n", latest.Version())
	},
}

// CheckForUpdatesBackground checks asynchronously if a new version is available.
func CheckForUpdatesBackground() {
	if version == "dev" {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		latest, found, err := selfupdate.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
		if err != nil || !found {
			return
		}

		if latest.GreaterThan(version) {
			fmt.Fprintf(os.Stderr, "\n💡 A new version of litellm-mux is available: %s (current: %s)\nRun `litellm-mux update` to upgrade.\n\n", latest.Version(), version)
		}
	}()
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
