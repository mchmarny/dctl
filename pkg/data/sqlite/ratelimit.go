package sqlite

import (
	"errors"
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

	jitter := time.Duration(rand.IntN(2000)) * time.Millisecond //nolint:gosec // jitter for rate limit backoff, not security-sensitive
	total := wait + jitter

	if resp.Rate.Remaining == 0 {
		slog.Warn("rate limit reached, pausing until reset",
			"reset_at", resetAt.Format(time.RFC3339),
			"wait", total.String(),
		)
	} else {
		slog.Warn("rate limit approaching, pausing until reset",
			"remaining", resp.Rate.Remaining,
			"reset_at", resetAt.Format(time.RFC3339),
			"wait", total.String(),
		)
	}

	time.Sleep(total)
}

// abuseRetryAfter returns the retry-after duration if the error is a secondary
// (abuse) rate limit error. Returns 0 if the error is not an abuse rate limit.
func abuseRetryAfter(err error) time.Duration {
	var abuse *github.AbuseRateLimitError
	if errors.As(err, &abuse) {
		d := abuse.GetRetryAfter()
		if d > 0 {
			return d
		}
		return 60 * time.Second
	}
	return 0
}
