package net

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

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
