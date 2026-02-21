package data

import (
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/go-github/v83/github"
)

const rateLimitThreshold = 10

func checkRateLimit(resp *github.Response) {
	if resp == nil {
		return
	}

	if resp.Rate.Remaining > rateLimitThreshold {
		return
	}

	resetAt := resp.Rate.Reset.Time
	wait := time.Until(resetAt)
	if wait <= 0 {
		return
	}

	jitter := time.Duration(rand.IntN(2000)) * time.Millisecond
	total := wait + jitter

	slog.Info("rate limit approaching, waiting",
		"remaining", resp.Rate.Remaining,
		"reset_at", resetAt.Format(time.RFC3339),
		"wait", total.String(),
	)

	time.Sleep(total)
}
