package main

import (
	"fmt"
	"os"
	"path"

	"github.com/mchmarny/dctl/pkg/auth"
	"github.com/urfave/cli/v2"
)

const (
	clientID      = "f1b500ebdf533aa8a3e2"
	tokenFileName = "github_token"
)

var (
	authCmd = &cli.Command{
		Name:    "auth",
		Aliases: []string{"a"},
		Usage:   "Authenticate to GitHub to obtain an access token",
		Action:  cmdInitAuthFlow,
	}
)

func cmdInitAuthFlow(c *cli.Context) error {
	code, err := auth.GetDeviceCode(clientID)
	if err != nil {
		return fmt.Errorf("failed to get device code: %w", err)
	}

	fmt.Printf("1). Copy this code: %s\n", code.UserCode)
	fmt.Printf("2). Navigate to this URL in your browser to authenticate: %s\n", code.VerificationURL)
	fmt.Print("3). Hit enter to complete the process:\n")
	fmt.Print(">")

	_, err = fmt.Scanln()
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	token, err := auth.GetToken(clientID, code)
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
	}

	if err = saveGitHubToken(token.AccessToken); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Printf("Token saved to: %s\n", path.Join(getHomeDir(), tokenFileName))

	return nil
}

func saveGitHubToken(token string) error {
	tokenPath := path.Join(getHomeDir(), tokenFileName)
	f, err := os.Create(tokenPath)
	if err != nil {
		return fmt.Errorf("failed to create token file: %s: %w", tokenPath, err)
	}
	defer f.Close()

	if _, err = f.WriteString(token); err != nil {
		return fmt.Errorf("failed to write token to file: %s: %w", tokenPath, err)
	}

	return nil
}

func getGitHubToken() (string, error) {
	tokenPath := path.Join(getHomeDir(), tokenFileName)
	b, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read token file: %s: %w", tokenPath, err)
	}

	return string(b), nil
}
