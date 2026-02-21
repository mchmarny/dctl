package data

import (
	"testing"
	"time"

	"github.com/google/go-github/v83/github"
)

func TestCheckRateLimit_Nil(t *testing.T) {
	// should not panic
	checkRateLimit(nil)
}

func TestCheckRateLimit_HighRemaining(t *testing.T) {
	resp := &github.Response{
		Rate: github.Rate{
			Remaining: 100,
			Limit:     5000,
			Reset:     github.Timestamp{Time: time.Now().Add(time.Hour)},
		},
	}
	// should return immediately without sleeping
	checkRateLimit(resp)
}

func TestCheckRateLimit_ResetInPast(t *testing.T) {
	resp := &github.Response{
		Rate: github.Rate{
			Remaining: 0,
			Limit:     5000,
			Reset:     github.Timestamp{Time: time.Now().Add(-time.Hour)},
		},
	}
	// Reset is in the past, should return immediately
	checkRateLimit(resp)
}
