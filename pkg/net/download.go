package net

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	maxIdleConns     = 10
	timeoutInSeconds = 60
	clientAgent      = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/100.0.4896.88 Safari/537.36"
)

var (
	reqTransport = &http.Transport{
		MaxIdleConns:          maxIdleConns,
		IdleConnTimeout:       timeoutInSeconds * time.Second,
		DisableCompression:    true,
		DisableKeepAlives:     false,
		ResponseHeaderTimeout: time.Duration(timeoutInSeconds) * time.Second,
	}
)

func getResp(url string) (resp *http.Response, err error) {
	c, err := GetHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP client: %w", err)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP Get request: %w", err)
	}

	req.Header.Set("User-Agent", clientAgent)

	return c.Do(req) //nolint:gosec // G704: URL from internal callers, not user input
}

var ErrorURLNotFound = errors.New("URL not found")

func Download(url string, filepath string) (retErr error) {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && retErr == nil {
			retErr = fmt.Errorf("closing file: %w", cerr)
		}
	}()

	resp, err := getResp(url)
	if err != nil {
		return fmt.Errorf("error creating HTTP Get request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrorURLNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error downloading file (status: %d - %s): %s", resp.StatusCode, resp.Status, url)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("error saving downloaded content to file: %w", err)
	}

	return nil
}
