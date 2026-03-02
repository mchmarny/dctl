package score

import (
	"fmt"
	"log/slog"
	"math"
)

// ModelVersion is the current scoring model version.
const ModelVersion = "3.2.0"

const (
	// Category weights (sum to 1.0).
	provenanceWeight  = 0.15
	ageWeight         = 0.15
	associationWeight = 0.05
	profileWeight     = 0.05
	proportionWeight  = 0.15
	recencyWeight     = 0.05
	prAcceptWeight    = 0.05
	followerWeight    = 0.05
	repoCountWeight   = 0.10
	burstWeight       = 0.10
	forkOnlyWeight    = 0.10

	// Ceilings and parameters.
	ageCeilDays              = 730
	verificationMaturityCeil = 730.0
	followerRatioCeil        = 10.0
	repoCountCeil            = 30.0
	baseHalfLifeDays         = 90.0
	minHalfLifeMultiple      = 0.25
	minProportionCeil        = 0.05
	minConfidenceCommits     = 30
	confCommitsPerContrib    = 10
	prCountCeil              = 20.0
	burstCeil                = 5.0
	forkOriginalCeil         = 5.0
)

// Exported category weights derived from signal constants above.
var (
	CategoryProvenanceWeight = provenanceWeight
	CategoryIdentityWeight   = ageWeight + associationWeight + profileWeight
	CategoryEngagementWeight = proportionWeight + recencyWeight + prAcceptWeight
	CategoryCommunityWeight  = followerWeight + repoCountWeight
	CategoryBehavioralWeight = burstWeight + forkOnlyWeight
)

// CategoryWeight describes a scoring category and its weight.
type CategoryWeight struct {
	Name   string  `json:"name" yaml:"name"`
	Weight float64 `json:"weight" yaml:"weight"`
}

// Signals holds the raw inputs to the reputation model.
type Signals struct {
	Suspended         bool  // Account suspended
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

	// v3 signals
	AuthorAssociation string // OWNER, MEMBER, COLLABORATOR, CONTRIBUTOR, FIRST_TIME_CONTRIBUTOR, NONE
	HasBio            bool   // Profile has bio
	HasCompany        bool   // Profile has company
	HasLocation       bool   // Profile has location
	HasWebsite        bool   // Profile has website/blog
	PRsMerged         int64  // Global merged PR count
	PRsClosed         int64  // Global closed-without-merge PR count
	RecentPRRepoCount int64  // Distinct repos with PR events in last 90 days
	ForkedRepos       int64  // Owned repos that are forks
	TrustedOrgMember  bool   // Member of a caller-specified trusted org
}

// Categories returns the model's scoring categories with their weights.
func Categories() []CategoryWeight {
	return []CategoryWeight{
		{Name: "code_provenance", Weight: CategoryProvenanceWeight},
		{Name: "identity", Weight: CategoryIdentityWeight},
		{Name: "engagement", Weight: CategoryEngagementWeight},
		{Name: "community", Weight: CategoryCommunityWeight},
		{Name: "behavioral", Weight: CategoryBehavioralWeight},
	}
}

// Compute returns a reputation score in [0.0, 1.0] using the v3 weighted model.
func Compute(s Signals) float64 {
	if s.Suspended {
		slog.Debug("score - suspended")
		return 0
	}

	var rep float64

	// --- Category 1: Code Provenance (0.15) ---
	if s.Commits > 0 && s.TotalCommits > 0 {
		verifiedRatio := float64(s.Commits-s.UnverifiedCommits) / float64(s.Commits)
		maturity := logCurve(float64(s.AgeDays), verificationMaturityCeil)
		rep += verifiedRatio * maturity * provenanceWeight
		slog.Debug(fmt.Sprintf("provenance: %.4f (verified=%.2f, maturity=%.2f)",
			verifiedRatio*maturity*provenanceWeight, verifiedRatio, maturity))
	}

	// --- Category 2: Identity (0.25) ---
	ageScore := logCurve(float64(s.AgeDays), ageCeilDays) * ageWeight
	rep += ageScore
	slog.Debug(fmt.Sprintf("age: %.4f (%d days)", ageScore, s.AgeDays))

	assocScore := associationScore(s.AuthorAssociation, s.OrgMember, s.TrustedOrgMember) * associationWeight
	rep += assocScore
	slog.Debug(fmt.Sprintf("association: %.4f (%s)", assocScore, s.AuthorAssociation))

	profileCount := 0
	if s.HasBio {
		profileCount++
	}
	if s.HasCompany {
		profileCount++
	}
	if s.HasLocation {
		profileCount++
	}
	if s.HasWebsite {
		profileCount++
	}
	profScore := float64(profileCount) / 4.0 * profileWeight
	rep += profScore
	slog.Debug(fmt.Sprintf("profile: %.4f (%d/4 fields)", profScore, profileCount))

	// --- Category 3: Engagement (0.25) ---
	if s.Commits > 0 && s.TotalCommits > 0 {
		proportion := float64(s.Commits) / float64(s.TotalCommits)
		propCeil := math.Max(1.0/float64(max(s.TotalContributors, 1)), minProportionCeil)

		confThreshold := float64(max(
			int64(s.TotalContributors)*int64(confCommitsPerContrib),
			int64(minConfidenceCommits),
		))
		confidence := math.Min(float64(s.TotalCommits)/confThreshold, 1.0)

		propScore := clampedRatio(proportion, propCeil) * confidence * proportionWeight
		rep += propScore
		slog.Debug(fmt.Sprintf("proportion: %.4f (prop=%.3f, ceil=%.3f, conf=%.3f)",
			propScore, proportion, propCeil, confidence))
	}

	numContrib := max(s.TotalContributors, 1)
	halfLifeMult := math.Max(1.0/math.Log(1+float64(numContrib)), minHalfLifeMultiple)
	if halfLifeMult > 1.0 {
		halfLifeMult = 1.0
	}
	halfLife := baseHalfLifeDays * halfLifeMult
	recScore := expDecay(float64(s.LastCommitDays), halfLife) * recencyWeight
	rep += recScore
	slog.Debug(fmt.Sprintf("recency: %.4f (%d days, halfLife=%.1f)",
		recScore, s.LastCommitDays, halfLife))

	totalTerminalPRs := s.PRsMerged + s.PRsClosed
	if totalTerminalPRs > 0 {
		mergeRate := float64(s.PRsMerged) / float64(totalTerminalPRs)
		confidence := logCurve(float64(totalTerminalPRs), prCountCeil)
		prScore := mergeRate * confidence * prAcceptWeight
		rep += prScore
		slog.Debug(fmt.Sprintf("pr_acceptance: %.4f (rate=%.2f, conf=%.2f)",
			prScore, mergeRate, confidence))
	}

	// --- Category 4: Community (0.15) ---
	if s.Following > 0 {
		ratio := float64(s.Followers) / float64(s.Following)
		rep += logCurve(ratio, followerRatioCeil) * followerWeight
		slog.Debug(fmt.Sprintf("followers: %.4f (ratio=%.2f)",
			logCurve(ratio, followerRatioCeil)*followerWeight, ratio))
	}

	totalRepos := float64(s.PublicRepos)
	rep += logCurve(totalRepos, repoCountCeil) * repoCountWeight
	slog.Debug(fmt.Sprintf("repos: %.4f (%d combined)",
		logCurve(totalRepos, repoCountCeil)*repoCountWeight, int64(totalRepos)))

	// --- Category 5: Behavioral (0.20) ---
	if s.RecentPRRepoCount > 0 && s.AgeDays > 0 {
		ageMonths := math.Max(float64(s.AgeDays)/30.0, 1.0)
		burstRate := float64(s.RecentPRRepoCount) / ageMonths
		bScore := (1.0 - clampedRatio(burstRate, burstCeil)) * burstWeight
		rep += bScore
		slog.Debug(fmt.Sprintf("burst: %.4f (rate=%.2f, repos=%d)",
			bScore, burstRate, s.RecentPRRepoCount))
	} else {
		rep += burstWeight
		slog.Debug(fmt.Sprintf("burst: %.4f (no recent PR repos)", burstWeight))
	}

	totalOwnedRepos := s.PublicRepos
	if totalOwnedRepos > 0 {
		originalRepos := float64(totalOwnedRepos - s.ForkedRepos)
		fScore := clampedRatio(originalRepos, forkOriginalCeil) * forkOnlyWeight
		rep += fScore
		slog.Debug(fmt.Sprintf("fork_ratio: %.4f (original=%.0f, forked=%d)",
			fScore, originalRepos, s.ForkedRepos))
	}

	result := toFixed(rep, 2)
	slog.Debug(fmt.Sprintf("reputation: %.2f", result))
	return result
}

// associationScore maps GitHub's author_association to a [0, 1] score.
// Falls back to OrgMember boolean when association is empty.
// When trustedOrgMember is true, the score is floored at COLLABORATOR level (0.8).
func associationScore(assoc string, orgMember, trustedOrgMember bool) float64 {
	var base float64
	switch assoc {
	case "OWNER", "MEMBER":
		base = 1.0
	case "COLLABORATOR":
		base = 0.8
	case "CONTRIBUTOR":
		base = 0.5
	case "FIRST_TIME_CONTRIBUTOR":
		base = 0.2
	case "NONE":
		base = 0.0
	default:
		if orgMember {
			base = 1.0
		}
	}

	if trustedOrgMember && base < 0.8 {
		base = 0.8
	}

	return base
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
