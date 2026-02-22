package score

import (
	"fmt"
	"log/slog"
	"math"
)

// ModelVersion is the current scoring model version.
const ModelVersion = "2.0.0"

const (
	// Category weights (sum to 1.0).
	provenanceWeight = 0.35
	ageWeight        = 0.15
	orgMemberWeight  = 0.10
	proportionWeight = 0.15
	recencyWeight    = 0.10
	followerWeight   = 0.10
	repoCountWeight  = 0.05

	// Ceilings and parameters.
	ageCeilDays           = 730
	followerRatioCeil     = 10.0
	repoCountCeil         = 30.0
	baseHalfLifeDays      = 90.0
	minHalfLifeMultiple   = 0.25
	provenanceFloor       = 0.1
	minProportionCeil     = 0.05
	minConfidenceCommits  = 30
	confCommitsPerContrib = 10
	noTFAMaxDiscount      = 0.5
)

// Exported category weights derived from signal constants above.
var (
	CategoryProvenanceWeight = provenanceWeight
	CategoryIdentityWeight   = ageWeight + orgMemberWeight
	CategoryEngagementWeight = proportionWeight + recencyWeight
	CategoryCommunityWeight  = followerWeight + repoCountWeight
)

// CategoryWeight describes a scoring category and its weight.
type CategoryWeight struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

// Signals holds the raw inputs to the reputation model.
type Signals struct {
	Suspended         bool  // Account suspended
	StrongAuth        bool  // 2FA enabled
	Commits           int64 // Author's commit count
	UnverifiedCommits int64 // Commits without verified signatures
	TotalCommits      int64 // Repo-wide total commits
	TotalContributors int   // Repo-wide contributor count
	AgeDays           int64 // Days since account creation
	OrgMember         bool  // Member of repo owner's org
	LastCommitDays    int64 // Days since most recent commit
	Followers         int64 // GitHub follower count
	Following         int64 // Following count (for ratio)
	PublicRepos       int64 // Public repository count
	PrivateRepos      int64 // Private repository count
}

// Categories returns the model's scoring categories with their weights.
func Categories() []CategoryWeight {
	return []CategoryWeight{
		{Name: "code_provenance", Weight: CategoryProvenanceWeight},
		{Name: "identity", Weight: CategoryIdentityWeight},
		{Name: "engagement", Weight: CategoryEngagementWeight},
		{Name: "community", Weight: CategoryCommunityWeight},
	}
}

// Compute returns a reputation score in [0.0, 1.0] using the v2 weighted model.
func Compute(s Signals) float64 {
	if s.Suspended {
		slog.Debug("score - suspended")
		return 0
	}

	var rep float64

	// --- Category 1: Code Provenance (0.35) ---
	if s.Commits > 0 && s.TotalCommits > 0 {
		verifiedRatio := float64(s.Commits-s.UnverifiedCommits) / float64(s.Commits)
		proportion := float64(s.Commits) / float64(s.TotalCommits)
		propCeil := math.Max(1.0/float64(max(s.TotalContributors, 1)), minProportionCeil)

		securityMultiplier := 1.0
		if !s.StrongAuth {
			securityMultiplier = 1.0 - noTFAMaxDiscount*clampedRatio(proportion, propCeil)
		}

		rawScore := verifiedRatio * securityMultiplier
		if s.StrongAuth && rawScore < provenanceFloor {
			rawScore = provenanceFloor
		}

		rep += rawScore * provenanceWeight
		slog.Debug(fmt.Sprintf("provenance: %.4f (verified=%.2f, multiplier=%.2f)",
			rep, verifiedRatio, securityMultiplier))
	} else if s.StrongAuth {
		rep += provenanceFloor * provenanceWeight
		slog.Debug(fmt.Sprintf("provenance: %.4f (2FA floor, no commits)", rep))
	}

	// --- Category 2: Identity Authenticity (0.25) ---
	rep += logCurve(float64(s.AgeDays), ageCeilDays) * ageWeight
	slog.Debug(fmt.Sprintf("age: %.4f (%d days)", rep, s.AgeDays))

	if s.OrgMember {
		rep += orgMemberWeight
	}
	slog.Debug(fmt.Sprintf("org: %.4f (member=%v)", rep, s.OrgMember))

	// --- Category 3: Engagement Depth (0.25) ---
	if s.Commits > 0 && s.TotalCommits > 0 {
		proportion := float64(s.Commits) / float64(s.TotalCommits)
		propCeil := math.Max(1.0/float64(max(s.TotalContributors, 1)), minProportionCeil)

		confThreshold := float64(max(
			int64(s.TotalContributors)*int64(confCommitsPerContrib),
			int64(minConfidenceCommits),
		))
		confidence := math.Min(float64(s.TotalCommits)/confThreshold, 1.0)

		rep += clampedRatio(proportion, propCeil) * confidence * proportionWeight
		slog.Debug(fmt.Sprintf("proportion: %.4f (prop=%.3f, ceil=%.3f, conf=%.3f)",
			rep, proportion, propCeil, confidence))
	}

	numContrib := max(s.TotalContributors, 1)
	halfLifeMult := math.Max(1.0/math.Log(1+float64(numContrib)), minHalfLifeMultiple)
	if halfLifeMult > 1.0 {
		halfLifeMult = 1.0
	}
	halfLife := baseHalfLifeDays * halfLifeMult
	rep += expDecay(float64(s.LastCommitDays), halfLife) * recencyWeight
	slog.Debug(fmt.Sprintf("recency: %.4f (%d days, halfLife=%.1f)",
		rep, s.LastCommitDays, halfLife))

	// --- Category 4: Community Standing (0.15) ---
	if s.Following > 0 {
		ratio := float64(s.Followers) / float64(s.Following)
		rep += logCurve(ratio, followerRatioCeil) * followerWeight
		slog.Debug(fmt.Sprintf("followers: %.4f (ratio=%.2f)", rep, ratio))
	}

	totalRepos := float64(s.PublicRepos + s.PrivateRepos)
	rep += logCurve(totalRepos, repoCountCeil) * repoCountWeight
	slog.Debug(fmt.Sprintf("repos: %.4f (%d combined)", rep, int64(totalRepos)))

	result := toFixed(rep, 2)
	slog.Debug(fmt.Sprintf("reputation: %.2f", result))
	return result
}

// clampedRatio maps val linearly into [0.0, 1.0] with ceil as the saturation point.
func clampedRatio(val, ceil float64) float64 {
	if ceil <= 0 || val <= 0 {
		return 0
	}
	if val >= ceil {
		return 1
	}
	return val / ceil
}

// logCurve maps val into [0.0, 1.0] with logarithmic diminishing returns.
func logCurve(val, ceil float64) float64 {
	if ceil <= 0 || val <= 0 {
		return 0
	}
	r := math.Log(1+val) / math.Log(1+ceil)
	if r >= 1 {
		return 1
	}
	return r
}

// expDecay models freshness with a half-life.
func expDecay(val, halfLife float64) float64 {
	if halfLife <= 0 {
		return 0
	}
	if val <= 0 {
		return 1
	}
	return math.Exp(-val * math.Ln2 / halfLife)
}

// toFixed truncates a float64 to the given precision.
func toFixed(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return float64(int(math.Round(num*output))) / output
}
