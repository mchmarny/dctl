package net

import (
	"context"
	"net/http"
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
