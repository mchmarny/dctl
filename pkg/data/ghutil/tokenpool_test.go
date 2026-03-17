package ghutil

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTokenPoolSingle(t *testing.T) {
	pool := NewTokenPool("tok1")
	assert.Equal(t, 1, pool.Size())
	assert.Equal(t, "tok1", pool.Token())
}

func TestNewTokenPoolMultiple(t *testing.T) {
	pool := NewTokenPool("tok1", "tok2", "tok3")
	assert.Equal(t, 3, pool.Size())
	// All start with 5000 remaining, first one wins.
	assert.Equal(t, "tok1", pool.Token())
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
}

func TestTokenSelectsHighestRemaining(t *testing.T) {
	pool := NewTokenPool("tok1", "tok2")
	pool.Update("tok1", 100, time.Now().Add(time.Hour))
	pool.Update("tok2", 4000, time.Now().Add(time.Hour))

	assert.Equal(t, "tok2", pool.Token())
}

func TestTokenFallsBackToEarliestReset(t *testing.T) {
	pool := NewTokenPool("tok1", "tok2")
	now := time.Now()
	pool.Update("tok1", 0, now.Add(30*time.Minute))
	pool.Update("tok2", 0, now.Add(10*time.Minute))

	assert.Equal(t, "tok2", pool.Token())
}

func TestTokenPrefersNonExhausted(t *testing.T) {
	pool := NewTokenPool("tok1", "tok2")
	pool.Update("tok1", 0, time.Now().Add(time.Hour))
	pool.Update("tok2", 500, time.Now().Add(time.Hour))

	assert.Equal(t, "tok2", pool.Token())
}

func TestUpdateUnknownTokenIsNoop(t *testing.T) {
	pool := NewTokenPool("tok1")
	pool.Update("unknown", 100, time.Now())
	assert.Equal(t, "tok1", pool.Token())
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
			pool.Update(tok, 1000, time.Now().Add(time.Hour))
		}()
	}
	wg.Wait()
}
