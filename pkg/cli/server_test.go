package cli

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "empty", in: "", want: ""},
		{name: "slash only", in: "/", want: ""},
		{name: "simple", in: "/devpulse", want: "/devpulse"},
		{name: "trailing slash", in: "/devpulse/", want: "/devpulse"},
		{name: "no leading slash", in: "devpulse", want: "/devpulse"},
		{name: "nested", in: "/apps/devpulse/", want: "/apps/devpulse"},
		{name: "spaces", in: "  /devpulse  ", want: "/devpulse"},
		{name: "path traversal", in: "/devpulse/../etc", wantErr: true},
		{name: "scheme injection", in: "http://evil.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeBasePath(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMakeRouterWithBasePath(t *testing.T) {
	mux := makeRouter(nil, "")
	assert.NotNil(t, mux)

	mux = makeRouter(nil, "/devpulse")
	assert.NotNil(t, mux)
}

func TestBasePathRouting(t *testing.T) {
	basePath := "/devpulse"
	mux := makeRouter(nil, basePath)
	handler := http.StripPrefix(basePath, mux)

	tests := []struct {
		name   string
		path   string
		status int
	}{
		{name: "root with base path", path: "/devpulse/", status: http.StatusOK},
		{name: "static with base path", path: "/devpulse/static/assets/css/app.css", status: http.StatusOK},
		{name: "favicon with base path", path: "/devpulse/favicon.ico", status: http.StatusOK},
		{name: "root without base path", path: "/", status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, tt.status, rec.Code)
		})
	}
}

func TestNoBasePathRouting(t *testing.T) {
	mux := makeRouter(nil, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}
