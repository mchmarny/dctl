package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsingBody(t *testing.T) {
	tests := map[string]int{
		"plain string with no name":     0,
		"@username up front":            1,
		"some username on the end @foo": 1,
		"@foo username on the end @bar": 2,
	}

	for input, expected := range tests {
		names := parseUsers(&input)
		assert.Len(t, names, expected)
	}
}
