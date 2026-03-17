package ghutil

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenPoolSingle(t *testing.T) {
	pool := NewTokenPool("tok1")
	assert.Equal(t, 1, pool.Size())
	assert.Equal(t, "tok1", pool.Token())
	assert.Equal(t, "tok1", pool.Token()) // single token always returns same
}

func TestNewTokenPoolMultiple(t *testing.T) {
	pool := NewTokenPool("tok1", "tok2", "tok3")
	assert.Equal(t, 3, pool.Size())
	assert.Equal(t, "tok1", pool.Token())
	assert.Equal(t, "tok2", pool.Token())
	assert.Equal(t, "tok3", pool.Token())
	assert.Equal(t, "tok1", pool.Token()) // wraps around
}

func TestNewTokenPoolCommaSeparated(t *testing.T) {
	pool := NewTokenPool("tok1,tok2,tok3")
	assert.Equal(t, 3, pool.Size())
}

func TestNewTokenPoolMixed(t *testing.T) {
	pool := NewTokenPool("tok1,tok2", "tok3")
	assert.Equal(t, 3, pool.Size())
}

func TestNewTokenPoolEmpty(t *testing.T) {
	pool := NewTokenPool("")
	assert.Equal(t, 0, pool.Size())
	assert.Equal(t, "", pool.Token())
}

func TestNewTokenPoolTrimsWhitespace(t *testing.T) {
	pool := NewTokenPool(" tok1 , tok2 ")
	assert.Equal(t, 2, pool.Size())
	assert.Equal(t, "tok1", pool.Token())
	assert.Equal(t, "tok2", pool.Token())
}

func TestTokenPoolRoundRobin(t *testing.T) {
	pool := NewTokenPool("a", "b")
	seen := make(map[string]int)
	for range 100 {
		seen[pool.Token()]++
	}
	assert.Equal(t, 50, seen["a"])
	assert.Equal(t, 50, seen["b"])
}

func TestTokenPoolConcurrentAccess(t *testing.T) {
	pool := NewTokenPool("tok1", "tok2", "tok3")

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tok := pool.Token()
			require.NotEmpty(t, tok)
		}()
	}
	wg.Wait()
}
