package net

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// GetHTTPClient returns a new HTTP client.
func GetHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cookie jar")
	}

	return &http.Client{
		Timeout:   time.Duration(timeoutInSeconds) * time.Second,
		Transport: reqTransport,
		Jar:       jar,
	}, nil
}

func GetOAuthClient(ctx context.Context, token string) *http.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{
			TokenType:   "token",
			AccessToken: token,
		},
	)
	tc := oauth2.NewClient(ctx, ts)

	return tc
}
