package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/mchmarny/devpulse/pkg/auth"
	"github.com/urfave/cli/v2"
	"github.com/zalando/go-keyring"
)

const (
	clientID       = "f1b500ebdf533aa8a3e2"
	tokenFileName  = "github_token"
	keyringService = "devpulse"
	keyringUser    = "github_token"
)

var (
	authCmd = &cli.Command{
		Name:            "auth",
		HideHelpCommand: true,
		Usage:           "Authenticate to GitHub to obtain an access token",
		Action:          cmdInitAuthFlow,
	}
)

func cmdInitAuthFlow(c *cli.Context) error {
	code, err := auth.GetDeviceCode(clientID)
	if err != nil {
		return fmt.Errorf("getting device code: %w", err)
	}

	fmt.Printf("1). Copy this code: %s\n", code.UserCode)
	fmt.Printf("2). Navigate to this URL in your browser to authenticate: %s\n", code.VerificationURL)
	fmt.Print("3). Hit enter to complete the process:\n")
	fmt.Print(">")

	if _, err = fmt.Scanln(); err != nil {
		return fmt.Errorf("reading user input: %w", err)
	}

	token, err := auth.GetToken(clientID, code)
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}

	if err = saveGitHubToken(token.AccessToken); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	fmt.Println("Token saved to OS keychain")
	return nil
}

func saveGitHubToken(token string) error {
	if err := keyring.Set(keyringService, keyringUser, token); err != nil {
		slog.Warn("keychain unavailable, falling back to file", "error", err)
		return saveGitHubTokenFile(token)
	}

	// Clean up legacy file if it exists
	legacyPath := path.Join(getHomeDir(), tokenFileName)
	os.Remove(legacyPath)

	return nil
}

func getGitHubToken() (string, error) {
	// Try keychain first
	token, err := keyring.Get(keyringService, keyringUser)
	if err == nil && token != "" {
		return token, nil
	}

	// Fall back to file
	token, err = getGitHubTokenFile()
	if err != nil {
		return "", err
	}

	// Migrate to keychain
	if migrateErr := keyring.Set(keyringService, keyringUser, token); migrateErr == nil {
		slog.Info("migrated token from file to OS keychain")
		legacyPath := path.Join(getHomeDir(), tokenFileName)
		os.Remove(legacyPath)
	}

	return token, nil
}

func saveGitHubTokenFile(token string) error {
	tokenPath := path.Join(getHomeDir(), tokenFileName)
	return os.WriteFile(tokenPath, []byte(token), 0600)
}

func getGitHubTokenFile() (string, error) {
	tokenPath := path.Join(getHomeDir(), tokenFileName)
	b, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("reading token file %s: %w", tokenPath, err)
	}
	return string(b), nil
}
