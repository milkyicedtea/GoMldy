package utils

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
	"strings"
)

func LoadEnv() {
	secretPath := "/run/secrets/"

	if _, err := os.Stat(secretPath); err == nil {
		fmt.Println("🔒 Loading environment variables from Docker secrets...")
		secrets := []string{"MODE", "MELODY_PSQL_URL", "RECAPTCHA_SECRET_KEY"}

		for _, secret := range secrets {
			// Is docker ever dev mode? Not for me, but if yes, just comment this :)
			if secret == "MODE" {
				continue
			}

			secretFilePath := filepath.Join(secretPath, secret)

			// Check if the specific secret file exists
			if _, err := os.Stat(secretFilePath); err == nil {
				// Read the secret value
				valueBytes, err := os.ReadFile(secretFilePath)
				if err == nil {
					// Set the environment variable
					value := strings.TrimSpace(string(valueBytes))
					err := os.Setenv(strings.ToUpper(secret), value)
					if err != nil {
						return
					}
				}
			}
		}
	} else {
		fmt.Println("🛠️ Loading environment variables from .env...")
		// Load from .env file
		err := godotenv.Load()
		if err != nil {
			return
		}
	}
}
