package data

import (
	"encoding/json"
	"time"
)

// ---------------------------------------------------------------------------
// Constants and configuration
// ---------------------------------------------------------------------------

const (
	DataFileName          string = "data.db"
	EventAgeMonthsDefault int    = 6

	EventTypePR           string = "pr"
	EventTypePRReview     string = "pr_review"
	EventTypeIssue        string = "issue"
	EventTypeIssueComment string = "issue_comment"
	EventTypeFork         string = "fork"
)

// UpdatableProperties lists developer fields that can be substituted.
var UpdatableProperties = []string{
	"entity",
}

// ---------------------------------------------------------------------------
// Database state types
// ---------------------------------------------------------------------------

type Query struct {
	On    int64  `json:"on,omitempty" yaml:"on,omitempty"`
	Type  string `json:"type,omitempty" yaml:"type,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
	Limit int    `json:"limit,omitempty" yaml:"limit,omitempty"`
}

type CountedResult struct {
	Query   Query            `json:"query,omitempty" yaml:"query,omitempty"`
	Results int              `json:"results,omitempty" yaml:"results,omitempty"`
	Data    map[string]int64 `json:"data,omitempty" yaml:"data,omitempty"`
}

type State struct {
	Since time.Time `json:"since" yaml:"since"`
	Page  int       `json:"page" yaml:"page"`
}

type DeleteResult struct {
	Org           string `json:"org" yaml:"org"`
	Repo          string `json:"repo" yaml:"repo"`
	Events        int64  `json:"events" yaml:"events"`
	RepoMeta      int64  `json:"repo_meta" yaml:"repo_meta"`
	Releases      int64  `json:"releases" yaml:"releases"`
	ReleaseAssets int64  `json:"release_assets" yaml:"release_assets"`
	State         int64  `json:"state" yaml:"state"`
}

// ---------------------------------------------------------------------------
// Substitution types
// ---------------------------------------------------------------------------

type Substitution struct {
	Prop    string `json:"prop" yaml:"prop"`
	Old     string `json:"old" yaml:"old"`
	New     string `json:"new" yaml:"new"`
	Records int64  `json:"records" yaml:"records"`
}

// ---------------------------------------------------------------------------
// Entity types
// ---------------------------------------------------------------------------

type EntityResult struct {
	Entity         string               `json:"entity,omitempty" yaml:"entity,omitempty"`
	DeveloperCount int                  `json:"developer_count,omitempty" yaml:"developerCount,omitempty"`
	Developers     []*DeveloperListItem `json:"developers,omitempty" yaml:"developers,omitempty"`
}

// ---------------------------------------------------------------------------
// Repo and org types
// ---------------------------------------------------------------------------

type CountedItem struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Count int    `json:"count,omitempty" yaml:"count,omitempty"`
}

type Repo struct {
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	FullName    string `json:"full_name,omitempty" yaml:"fullName,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
}

type ListItem struct {
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
	Text  string `json:"text,omitempty" yaml:"text,omitempty"`
	Type  string `json:"type,omitempty" yaml:"type,omitempty"`
}

type Org struct {
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Company     string `json:"company,omitempty" yaml:"company,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type OrgRepoItem struct {
	Org  string `json:"org,omitempty" yaml:"org,omitempty"`
	Repo string `json:"repo,omitempty" yaml:"repo,omitempty"`
}

// ---------------------------------------------------------------------------
// Developer types
// ---------------------------------------------------------------------------

type Developer struct {
	Username      string `json:"username,omitempty" yaml:"username,omitempty"`
	FullName      string `json:"full_name,omitempty" yaml:"fullName,omitempty"`
	Email         string `json:"email,omitempty" yaml:"email,omitempty"`
	AvatarURL     string `json:"avatar,omitempty" yaml:"avatar,omitempty"`
	ProfileURL    string `json:"url,omitempty" yaml:"url,omitempty"`
	Entity        string `json:"entity,omitempty" yaml:"entity,omitempty"`
	Organizations []*Org `json:"organizations,omitempty" yaml:"organizations,omitempty"`
}

type DeveloperListItem struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Entity   string `json:"entity,omitempty" yaml:"entity,omitempty"`
}

// ---------------------------------------------------------------------------
// Event types
// ---------------------------------------------------------------------------

type Event struct {
	Org          string  `json:"org,omitempty" yaml:"org,omitempty"`
	Repo         string  `json:"repo,omitempty" yaml:"repo,omitempty"`
	Username     string  `json:"username,omitempty" yaml:"username,omitempty"`
	Type         string  `json:"type,omitempty" yaml:"type,omitempty"`
	Date         string  `json:"date,omitempty" yaml:"date,omitempty"`
	URL          string  `json:"url,omitempty" yaml:"url,omitempty"`
	Mentions     string  `json:"mentions,omitempty" yaml:"mentions,omitempty"`
	Labels       string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	State        *string `json:"state,omitempty" yaml:"state,omitempty"`
	Number       *int    `json:"number,omitempty" yaml:"number,omitempty"`
	CreatedAt    *string `json:"created_at,omitempty" yaml:"createdAt,omitempty"`
	ClosedAt     *string `json:"closed_at,omitempty" yaml:"closedAt,omitempty"`
	MergedAt     *string `json:"merged_at,omitempty" yaml:"mergedAt,omitempty"`
	Additions    *int    `json:"additions,omitempty" yaml:"additions,omitempty"`
	Deletions    *int    `json:"deletions,omitempty" yaml:"deletions,omitempty"`
	ChangedFiles *int    `json:"changed_files,omitempty" yaml:"changed_files,omitempty"`
	Commits      *int    `json:"commits,omitempty" yaml:"commits,omitempty"`
	Title        string  `json:"title,omitempty" yaml:"title,omitempty"`
}

// ImportSummary contains per-repo import metadata.
type ImportSummary struct {
	Repo       string `json:"repo" yaml:"repo"`
	Since      string `json:"since" yaml:"since"`
	Events     int    `json:"events" yaml:"events"`
	Developers int    `json:"developers" yaml:"developers"`
}

// ---------------------------------------------------------------------------
// Event query types
// ---------------------------------------------------------------------------

type EventTypeSeries struct {
	Dates         []string  `json:"dates" yaml:"dates"`
	PRs           []int     `json:"pr" yaml:"pr"`
	PRReviews     []int     `json:"pr_review" yaml:"prReview"`
	Issues        []int     `json:"issue" yaml:"issue"`
	IssueComments []int     `json:"issue_comment" yaml:"issueComment"`
	Forks         []int     `json:"fork" yaml:"fork"`
	Total         []int     `json:"total" yaml:"total"`
	Trend         []float32 `json:"trend" yaml:"trend"`
}

type EventDetails struct {
	Event     *Event     `json:"event,omitempty" yaml:"event,omitempty"`
	Developer *Developer `json:"developer,omitempty" yaml:"developer,omitempty"`
}

type EventSearchCriteria struct {
	FromDate *string `json:"from,omitempty" yaml:"from,omitempty"`
	ToDate   *string `json:"to,omitempty" yaml:"to,omitempty"`
	Type     *string `json:"type,omitempty" yaml:"type,omitempty"`
	Org      *string `json:"org,omitempty" yaml:"org,omitempty"`
	Repo     *string `json:"repo,omitempty" yaml:"repo,omitempty"`
	Username *string `json:"user,omitempty" yaml:"user,omitempty"`
	Entity   *string `json:"entity,omitempty" yaml:"entity,omitempty"`
	Mention  *string `json:"mention,omitempty" yaml:"mention,omitempty"`
	Label    *string `json:"label,omitempty" yaml:"label,omitempty"`
	Page     int     `json:"page,omitempty" yaml:"page,omitempty"`
	PageSize int     `json:"page_size,omitempty" yaml:"pageSize,omitempty"`
}

func (c EventSearchCriteria) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Insights series types
// ---------------------------------------------------------------------------

type InsightsSummary struct {
	BusFactor    int    `json:"bus_factor" yaml:"busFactor"`
	PonyFactor   int    `json:"pony_factor" yaml:"ponyFactor"`
	Orgs         int    `json:"orgs" yaml:"orgs"`
	Repos        int    `json:"repos" yaml:"repos"`
	Events       int    `json:"events" yaml:"events"`
	Contributors int    `json:"contributors" yaml:"contributors"`
	LastImport   string `json:"last_import" yaml:"lastImport"`
}

type DailyActivitySeries struct {
	Dates  []string `json:"dates"`
	Counts []int    `json:"counts"`
}

type VelocitySeries struct {
	Months  []string  `json:"months" yaml:"months"`
	Count   []int     `json:"count" yaml:"count"`
	AvgDays []float64 `json:"avg_days" yaml:"avgDays"`
}

type IssueRatioSeries struct {
	Months []string `json:"months" yaml:"months"`
	Opened []int    `json:"opened" yaml:"opened"`
	Closed []int    `json:"closed" yaml:"closed"`
}

type FirstResponseSeries struct {
	Months   []string  `json:"months" yaml:"months"`
	IssueAvg []float64 `json:"issue_avg" yaml:"issueAvg"`
	PRAvg    []float64 `json:"pr_avg" yaml:"prAvg"`
}

type RetentionSeries struct {
	Months    []string `json:"months" yaml:"months"`
	New       []int    `json:"new" yaml:"new"`
	Returning []int    `json:"returning" yaml:"returning"`
}

type PRReviewRatioSeries struct {
	Months  []string  `json:"months" yaml:"months"`
	PRs     []int     `json:"prs" yaml:"prs"`
	Reviews []int     `json:"reviews" yaml:"reviews"`
	Ratio   []float64 `json:"ratio" yaml:"ratio"`
}

type ChangeFailureRateSeries struct {
	Months      []string  `json:"months" yaml:"months"`
	Failures    []int     `json:"failures" yaml:"failures"`
	Deployments []int     `json:"deployments" yaml:"deployments"`
	Rate        []float64 `json:"rate" yaml:"rate"`
}

type ReviewLatencySeries struct {
	Months   []string  `json:"months" yaml:"months"`
	Count    []int     `json:"count" yaml:"count"`
	AvgHours []float64 `json:"avg_hours" yaml:"avgHours"`
}

type PRSizeSeries struct {
	Months []string `json:"months" yaml:"months"`
	Small  []int    `json:"small" yaml:"small"`
	Medium []int    `json:"medium" yaml:"medium"`
	Large  []int    `json:"large" yaml:"large"`
	XLarge []int    `json:"xlarge" yaml:"xlarge"`
}

type MomentumSeries struct {
	Months []string `json:"months" yaml:"months"`
	Active []int    `json:"active" yaml:"active"`
	Delta  []int    `json:"delta" yaml:"delta"`
}

type ForksAndActivitySeries struct {
	Months []string `json:"months" yaml:"months"`
	Forks  []int    `json:"forks" yaml:"forks"`
	Events []int    `json:"events" yaml:"events"`
}

type ContributorFunnelSeries struct {
	Months       []string `json:"months" yaml:"months"`
	FirstComment []int    `json:"first_comment" yaml:"firstComment"`
	FirstPR      []int    `json:"first_pr" yaml:"firstPR"`
	FirstMerge   []int    `json:"first_merge" yaml:"firstMerge"`
}

type ContributorProfileSeries struct {
	Metrics    []string  `json:"metrics"`
	Values     []int     `json:"values"`
	Averages   []float64 `json:"averages"`
	Reputation *float64  `json:"reputation,omitempty"`
}

// ---------------------------------------------------------------------------
// Release series types
// ---------------------------------------------------------------------------

type ReleaseCadenceSeries struct {
	Months      []string `json:"months" yaml:"months"`
	Total       []int    `json:"total" yaml:"total"`
	Stable      []int    `json:"stable" yaml:"stable"`
	Deployments []int    `json:"deployments" yaml:"deployments"`
}

type ReleaseDownloadsSeries struct {
	Months    []string `json:"months" yaml:"months"`
	Downloads []int    `json:"downloads" yaml:"downloads"`
}

type ReleaseDownloadsByTagSeries struct {
	Tags      []string `json:"tags" yaml:"tags"`
	Downloads []int    `json:"downloads" yaml:"downloads"`
}

// ---------------------------------------------------------------------------
// Container series types
// ---------------------------------------------------------------------------

// ContainerActivitySeries is the chart data for container version publishes per month.
type ContainerActivitySeries struct {
	Months   []string `json:"months" yaml:"months"`
	Versions []int    `json:"versions" yaml:"versions"`
}

// ---------------------------------------------------------------------------
// Reputation types
// ---------------------------------------------------------------------------

// ReputationResult is returned by the shallow bulk import.
type ReputationResult struct {
	Updated int `json:"updated" yaml:"updated"`
	Skipped int `json:"skipped" yaml:"skipped"`
	Errors  int `json:"errors" yaml:"errors"`
}

// DeepReputationResult is returned by the bulk deep scoring step.
type DeepReputationResult struct {
	Scored  int `json:"scored" yaml:"scored"`
	Skipped int `json:"skipped" yaml:"skipped"`
	Errors  int `json:"errors" yaml:"errors"`
}

// ReputationDistribution is the dashboard chart data.
type ReputationDistribution struct {
	Labels []string  `json:"labels" yaml:"labels"`
	Data   []float64 `json:"data" yaml:"data"`
	Scored int       `json:"scored" yaml:"scored"`
	Total  int       `json:"total" yaml:"total"`
}

// UserReputation is returned by the on-demand deep score endpoint.
type UserReputation struct {
	Username   string         `json:"username" yaml:"username"`
	Reputation float64        `json:"reputation" yaml:"reputation"`
	Deep       bool           `json:"deep" yaml:"deep"`
	Signals    *SignalSummary `json:"signals,omitempty" yaml:"signals,omitempty"`
}

// SignalSummary exposes gathered signals to the UI.
type SignalSummary struct {
	AgeDays           int64  `json:"age_days" yaml:"ageDays"`
	Followers         int64  `json:"followers" yaml:"followers"`
	Following         int64  `json:"following" yaml:"following"`
	PublicRepos       int64  `json:"public_repos" yaml:"publicRepos"`
	Suspended         bool   `json:"suspended" yaml:"suspended"`
	OrgMember         bool   `json:"org_member" yaml:"orgMember"`
	Commits           int64  `json:"commits" yaml:"commits"`
	TotalCommits      int64  `json:"total_commits" yaml:"totalCommits"`
	TotalContributors int    `json:"total_contributors" yaml:"totalContributors"`
	LastCommitDays    int64  `json:"last_commit_days" yaml:"lastCommitDays"`
	AuthorAssociation string `json:"author_association" yaml:"authorAssociation"`
	HasBio            bool   `json:"has_bio" yaml:"hasBio"`
	HasCompany        bool   `json:"has_company" yaml:"hasCompany"`
	HasLocation       bool   `json:"has_location" yaml:"hasLocation"`
	HasWebsite        bool   `json:"has_website" yaml:"hasWebsite"`
	PRsMerged         int64  `json:"prs_merged" yaml:"prsMerged"`
	PRsClosed         int64  `json:"prs_closed" yaml:"prsClosed"`
	RecentPRRepoCount int64  `json:"recent_pr_repo_count" yaml:"recentPRRepoCount"`
	ForkedRepos       int64  `json:"forked_repos" yaml:"forkedRepos"`
	TrustedOrgMember  bool   `json:"trusted_org_member" yaml:"trustedOrgMember"`
}

// ---------------------------------------------------------------------------
// CNCF affiliation types
// ---------------------------------------------------------------------------

type CNCFDeveloper struct {
	Username     string             `json:"username,omitempty" yaml:"username,omitempty"`
	Identities   []string           `json:"identities,omitempty" yaml:"identities,omitempty"`
	Affiliations []*CNCFAffiliation `json:"affiliations,omitempty" yaml:"affiliations,omitempty"`
}

func (c *CNCFDeveloper) GetBestIdentity() string {
	if len(c.Identities) == 0 {
		return ""
	}

	return c.Identities[0] // TODO: use regex to ensure a valid email address
}

func (c *CNCFDeveloper) GetLatestAffiliation() string {
	if len(c.Affiliations) == 0 {
		return ""
	}

	lastFrom := &CNCFAffiliation{From: "0000-00-00"}
	for _, a := range c.Affiliations {
		if a.From > lastFrom.From {
			lastFrom = a
		}
	}
	return lastFrom.Entity
}

type CNCFAffiliation struct {
	Entity string `json:"entity,omitempty" yaml:"entity,omitempty"`
	From   string `json:"from,omitempty" yaml:"from,omitempty"`
	To     string `json:"to,omitempty" yaml:"to,omitempty"`
}

type AffiliationImportResult struct {
	Duration    string `json:"duration,omitempty" yaml:"duration,omitempty"`
	DBDevs      int    `json:"db_devs,omitempty" yaml:"dbDevs,omitempty"`
	CNCFDevs    int    `json:"cncf_devs,omitempty" yaml:"cncfDevs,omitempty"`
	MappedDevs  int    `json:"mapped_devs,omitempty" yaml:"mappedDevs,omitempty"`
	SkippedDevs int    `json:"skipped_devs,omitempty" yaml:"skippedDevs,omitempty"`
}

// ---------------------------------------------------------------------------
// Repo metric history types
// ---------------------------------------------------------------------------

type RepoMetricHistory struct {
	Org   string `json:"org"`
	Repo  string `json:"repo"`
	Date  string `json:"date"`
	Stars int    `json:"stars"`
	Forks int    `json:"forks"`
}

// ---------------------------------------------------------------------------
// Repo metadata types
// ---------------------------------------------------------------------------

type RepoMeta struct {
	Org                string `json:"org" yaml:"org"`
	Repo               string `json:"repo" yaml:"repo"`
	Stars              int    `json:"stars" yaml:"stars"`
	Forks              int    `json:"forks" yaml:"forks"`
	OpenIssues         int    `json:"open_issues" yaml:"openIssues"`
	Language           string `json:"language" yaml:"language"`
	License            string `json:"license" yaml:"license"`
	Archived           bool   `json:"archived" yaml:"archived"`
	HasCoC             bool   `json:"has_coc" yaml:"hasCoc"`
	HasContributing    bool   `json:"has_contributing" yaml:"hasContributing"`
	HasReadme          bool   `json:"has_readme" yaml:"hasReadme"`
	HasIssueTemplate   bool   `json:"has_issue_template" yaml:"hasIssueTemplate"`
	HasPRTemplate      bool   `json:"has_pr_template" yaml:"hasPrTemplate"`
	CommunityHealthPct int    `json:"community_health_pct" yaml:"communityHealthPct"`
	UpdatedAt          string `json:"updated_at" yaml:"updatedAt"`
}

type RepoOverview struct {
	Org          string `json:"org"`
	Repo         string `json:"repo"`
	Stars        int    `json:"stars"`
	Forks        int    `json:"forks"`
	OpenIssues   int    `json:"open_issues"`
	Events       int    `json:"events"`
	Contributors int    `json:"contributors"`
	Scored       int    `json:"scored"`
	Language     string `json:"language"`
	License      string `json:"license"`
	Archived     bool   `json:"archived"`
	LastImport   string `json:"last_import"`
}
