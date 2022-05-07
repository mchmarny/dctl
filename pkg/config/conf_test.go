package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testDir = "./"
)

func TestConfig(t *testing.T) {
	c1, err := ReadOrCreate(testDir)
	assert.NoError(t, err)
	assert.NotNil(t, c1)

	c1.Int = 2
	c1.Bool = true
	c1.Value = "test"

	err = Save(testDir, c1)
	assert.NoError(t, err)

	c2, err := ReadOrCreate(testDir)
	assert.NoError(t, err)
	assert.NotNil(t, c2)
	assert.Equal(t, c1.Int, c2.Int)
	assert.Equal(t, c1.Bool, c2.Bool)
	assert.Equal(t, c1.Value, c2.Value)
}
