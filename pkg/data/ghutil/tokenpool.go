package ghutil

import (
	"strings"
	"sync"
	"time"
)

// TokenPool manages a pool of GitHub API tokens, selecting the one with the
// most remaining quota for each request. Safe for concurrent use.
// Works transparently with a single token.
type TokenPool struct {
	mu      sync.Mutex
	entries []tokenEntry
}

type tokenEntry struct {
	token     string
	remaining int
	resetAt   time.Time
}

// NewTokenPool creates a pool from one or more tokens. Tokens can be passed
// individually or as a single comma-separated string.
func NewTokenPool(tokens ...string) *TokenPool {
	var entries []tokenEntry
	for _, t := range tokens {
		for _, part := range strings.Split(t, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				entries = append(entries, tokenEntry{
					token:     part,
					remaining: 5000, // assume full quota initially
				})
			}
		}
	}
	return &TokenPool{entries: entries}
}

// Token returns the token with the most remaining quota. If all tokens are
// exhausted, it returns the one whose reset window expires soonest.
func (p *TokenPool) Token() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.entries) == 0 {
		return ""
	}
	if len(p.entries) == 1 {
		return p.entries[0].token
	}

	now := time.Now()
	best := 0
	for i := 1; i < len(p.entries); i++ {
		if p.better(i, best, now) {
			best = i
		}
	}
	return p.entries[best].token
}

// Update records the rate limit state for a token after an API response.
func (p *TokenPool) Update(token string, remaining int, resetAt time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i := range p.entries {
		if p.entries[i].token == token {
			p.entries[i].remaining = remaining
			p.entries[i].resetAt = resetAt
			return
		}
	}
}

// Size returns the number of tokens in the pool.
func (p *TokenPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

// better returns true if entry i is a better choice than entry j.
func (p *TokenPool) better(i, j int, now time.Time) bool {
	ei, ej := p.entries[i], p.entries[j]

	// If both have quota, pick the one with more remaining.
	if ei.remaining > 0 && ej.remaining > 0 {
		return ei.remaining > ej.remaining
	}

	// If one is exhausted and the other isn't, pick the non-exhausted one.
	if ei.remaining > 0 {
		return true
	}
	if ej.remaining > 0 {
		return false
	}

	// Both exhausted — pick the one that resets sooner.
	return ei.resetAt.Before(ej.resetAt)
}
