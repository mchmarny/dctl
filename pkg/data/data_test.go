package data

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testDir = "./"
)

func deleteDB() {
	os.Remove("./data.db")
}

func TestData(t *testing.T) {
	deleteDB()

	err := Init(testDir)
	assert.NoError(t, err)

	ids := make([]int64, 0)
	for i := int64(1); i <= 10; i++ {
		ids = append(ids, i)
	}

	err = SaveAll(ids)
	assert.NoError(t, err)

	ids2, err := Query(0)
	assert.NoError(t, err)
	assert.Equal(t, len(ids), len(ids2))

	val, err := Get(ids[0])
	assert.NoError(t, err)
	assert.NotNil(t, val)

	err = Close()
	assert.NoError(t, err)

	deleteDB()
}
