package net

import (
	"encoding/json"
	"fmt"
)

// GetJSON retreaves the HTTP content and decodes it into the passed target.
func GetJSON[T any](url string, target *T) error {
	resp, err := getResp(url)
	if err != nil {
		return fmt.Errorf("error creating HTTP Get request: %w", err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("error decoding content: %w", err)
	}
	return nil
}
