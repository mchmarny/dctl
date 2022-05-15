package data

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanEntity(t *testing.T) {
	var err error
	entityRegEx, err = regexp.Compile(nonAlphaNumRegex)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"Google LLC":           "GOOGLE",
		"Hitachi Vantara LLC":  "HITACHI VANTARA",
		"MAX KELSEN PTY. LTD.": "MAX KELSEN PTY",
		"Mercari Inc":          "MERCARI",
		"Some Company Corp.":   "SOME",
		"Big Cars LLC.":        "BIG CARS",
		"International Business Machines Corporation": "IBM",
	}

	for input, expected := range tests {
		val := cleanEntityName(input)
		assert.Equal(t, expected, val)
	}
}
