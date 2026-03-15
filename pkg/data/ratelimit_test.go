package data

import (
	"testing"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/stretchr/testify/assert"
)

func TestCheckRateLimit_Nil(t *testing.T) {
	start := time.Now()
	checkRateLimit(nil)
	assert.Less(t, time.Since(start), time.Second, "nil response should return immediately")
}

func TestCheckRateLimit_HighRemaining(t *testing.T) {
	resp := &github.Response{
		Rate: github.Rate{
			Remaining: 100,
			Limit:     5000,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Hour)},
		},
	}
	start := time.Now()
	checkRateLimit(resp)
	assert.Less(t, time.Since(start), time.Second, "high remaining should not sleep")
}

func TestCheckRateLimit_ResetInPast(t *testing.T) {
	resp := &github.Response{
		Rate: github.Rate{
			Remaining: 0,
			Limit:     5000,
			Reset:     github.Timestamp{Time: time.Now().Add(-time.Hour)},
		},
	}
	start := time.Now()
	checkRateLimit(resp)
	assert.Less(t, time.Since(start), time.Second, "past reset should not sleep")
}
