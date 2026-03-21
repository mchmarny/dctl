package net

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHTTPClient(t *testing.T) {
	client, err := GetHTTPClient()
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Jar)
}

func TestGetOAuthClient(t *testing.T) {
	ctx := context.Background()
	client := GetOAuthClient(ctx, "test-token")
	assert.NotNil(t, client)
}

func TestGetOAuthClientSetsAuthHeader(t *testing.T) {
	ctx := context.Background()

	// Start a test server that echoes the Authorization header.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{
			name:  "single token",
			token: "ghp_abc123",
			want:  "token ghp_abc123",
		},
		{
			name:  "comma-separated tokens produce invalid header",
			token: "ghp_abc123,ghp_def456",
			want:  "token ghp_abc123,ghp_def456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := GetOAuthClient(ctx, tc.token)
			resp, err := client.Get(srv.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(body))
		})
	}
}

func TestPrintHTTPResponse_Nil(t *testing.T) {
	// should not panic
	PrintHTTPResponse(nil)
}

func TestPrintHTTPResponse_WithResponse(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       http.NoBody,
	}
	// should not panic
	PrintHTTPResponse(resp)
}
