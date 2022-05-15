package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"
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

func getResp(url string) (resp *http.Response, err error) {
	c := http.Client{
		Timeout:   time.Duration(timeoutInSeconds) * time.Second,
		Transport: reqTransport,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error creating HTTP Get request")
	}

	req.Header.Set("User-Agent", clientAgent)

	return c.Do(req)
}
