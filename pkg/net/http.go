package net

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// GetJSON retreaves the HTTP content and decodes it into the passed target.
func GetJSON[T any](url string, target *T) error {
	resp, err := getResp(url)
	if err != nil {
		return errors.Wrap(err, "error creating HTTP Get request")
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return errors.Wrap(err, "error decodding content")
	}
	return nil
}
