package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCNCFDeveloper_GetBestIdentity(t *testing.T) {
	dev := &CNCFDeveloper{Identities: []string{"first@test.com", "second@test.com"}}
	assert.Equal(t, "first@test.com", dev.GetBestIdentity())

	empty := &CNCFDeveloper{}
	assert.Equal(t, "", empty.GetBestIdentity())
}

func TestCNCFDeveloper_GetLatestAffiliation(t *testing.T) {
	dev := &CNCFDeveloper{
		Affiliations: []*CNCFAffiliation{
			{Entity: "OldCorp", From: "2020-01-01"},
			{Entity: "NewCorp", From: "2023-06-01"},
		},
	}
	assert.Equal(t, "NewCorp", dev.GetLatestAffiliation())

	empty := &CNCFDeveloper{}
	assert.Equal(t, "", empty.GetLatestAffiliation())
}

func TestCNCFDeveloper_GetLatestAffiliation_Single(t *testing.T) {
	dev := &CNCFDeveloper{
		Affiliations: []*CNCFAffiliation{
			{Entity: "OnlyCorp", From: "2023-01-01"},
		},
	}
	assert.Equal(t, "OnlyCorp", dev.GetLatestAffiliation())
}
