package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeBasePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "slash only", in: "/", want: ""},
		{name: "simple", in: "/devpulse", want: "/devpulse"},
		{name: "trailing slash", in: "/devpulse/", want: "/devpulse"},
		{name: "no leading slash", in: "devpulse", want: "/devpulse"},
		{name: "nested", in: "/apps/devpulse/", want: "/apps/devpulse"},
		{name: "spaces", in: "  /devpulse  ", want: "/devpulse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeBasePath(tt.in))
		})
	}
}

func TestMakeRouterWithBasePath(t *testing.T) {
	// Verify makeRouter does not panic with empty or non-empty base path.
	mux := makeRouter(nil, "")
	assert.NotNil(t, mux)

	mux = makeRouter(nil, "/devpulse")
	assert.NotNil(t, mux)
}
