package ghutil

import (
	"strings"
	"sync"
)

// TokenPool manages a pool of GitHub API tokens using round-robin selection.
// Safe for concurrent use. Works transparently with a single token.
type TokenPool struct {
	mu      sync.Mutex
	tokens  []string
	current int
}

// NewTokenPool creates a pool from one or more tokens. Tokens can be passed
// individually or as a single comma-separated string.
func NewTokenPool(tokens ...string) *TokenPool {
	var list []string
	for _, t := range tokens {
		for _, part := range strings.Split(t, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				list = append(list, part)
			}
		}
	}
	return &TokenPool{tokens: list}
}

// Token returns the next token in the round-robin rotation.
func (p *TokenPool) Token() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.tokens) == 0 {
		return ""
	}

	tok := p.tokens[p.current]
	p.current = (p.current + 1) % len(p.tokens)
	return tok
}

// Size returns the number of tokens in the pool.
func (p *TokenPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.tokens)
}
