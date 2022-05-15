package net

import (
	"io"
	"net/http"
	"os"
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

var ErrorURLNotFound = errors.New("URL not found")

func Download(url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := getResp(url)
	if err != nil {
		return errors.Wrap(err, "error creating HTTP Get request")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrorURLNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("error downloading file (status: %d - %s): %s", resp.StatusCode, resp.Status, url)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return errors.Wrap(err, "error saving downloaded content to file")
	}

	return nil
}
