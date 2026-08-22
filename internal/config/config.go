package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	LiteLLMURL string
	MasterKey  string
}

func LoadConfig(flagURL, flagMasterKey string) (*Config, error) {
	// 1. Load .env file into environment if not already set
	if file, err := os.Open(".env"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
		}
	}

	// Precedence: CLI Flag > Environment Variable (.env or system env)
	url := flagURL
	if url == "" {
		url = os.Getenv("LITELLM_URL")
	}

	masterKey := flagMasterKey
	if masterKey == "" {
		masterKey = os.Getenv("LITELLM_MASTER_KEY")
		if masterKey == "" {
			masterKey = os.Getenv("MASTER_KEY")
		}
	}

	if url == "" {
		return nil, fmt.Errorf("LiteLLM URL ist nicht angegeben (weder per --url, LITELLM_URL noch in .env)")
	}
	if masterKey == "" {
		return nil, fmt.Errorf("LiteLLM Master Key ist nicht angegeben (weder per --master-key, LITELLM_MASTER_KEY/MASTER_KEY noch in .env)")
	}

	return &Config{
		LiteLLMURL: strings.TrimRight(url, "/"),
		MasterKey:  masterKey,
	}, nil
}
