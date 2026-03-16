package data

import (
	"context"
	"io"
	"net/http"
	"time"
)

// StateStore manages import state tracking.
type StateStore interface {
	GetState(query, org, repo string, min time.Time) (*State, error)
	SaveState(query, org, repo string, state *State) error
	ClearState(org, repo string) error
	GetDataState() (map[string]int64, error)
}

// DeleteStore manages data deletion.
type DeleteStore interface {
	DeleteRepoData(org, repo string) (*DeleteResult, error)
}

// SubstitutionStore manages developer substitutions.
type SubstitutionStore interface {
	SaveAndApplyDeveloperSub(prop, old, new string) (*Substitution, error)
	ApplySubstitutions() ([]*Substitution, error)
}

// EntityStore manages entity lookups and queries.
type EntityStore interface {
	GetEntityLike(query string, limit int) ([]*ListItem, error)
	GetEntity(val string) (*EntityResult, error)
	QueryEntities(val string, limit int) ([]*CountedItem, error)
	CleanEntities() error
}

// RepoStore manages repository lookups.
type RepoStore interface {
	GetRepoLike(query string, limit int) ([]*ListItem, error)
}

// OrgStore manages organization-level queries.
type OrgStore interface {
	GetAllOrgRepos() ([]*OrgRepoItem, error)
	GetDeveloperPercentages(entity, org, repo *string, ex []string, months int) ([]*CountedItem, error)
	GetEntityPercentages(entity, org, repo *string, ex []string, months int) ([]*CountedItem, error)
	SearchDeveloperUsernames(query string, org, repo *string, months, limit int) ([]string, error)
	GetOrgLike(query string, limit int) ([]*ListItem, error)
}

// DeveloperStore manages developer records.
type DeveloperStore interface {
	GetDeveloperUsernames() ([]string, error)
	GetNoFullnameDeveloperUsernames() ([]string, error)
	SaveDevelopers(devs []*Developer) error
	MergeDeveloper(ctx context.Context, client *http.Client, username string, cDev *CNCFDeveloper) (*Developer, error)
	GetDeveloper(username string) (*Developer, error)
	SearchDevelopers(val string, limit int) ([]*DeveloperListItem, error)
	UpdateDeveloperNames(devs map[string]string) error
}

// QueryStore manages event search and aggregation queries.
type QueryStore interface {
	SearchEvents(q *EventSearchCriteria) ([]*EventDetails, error)
	GetMinEventDate(org, repo *string) (string, error)
	GetEventTypeSeries(org, repo, entity *string, months int) (*EventTypeSeries, error)
}

// EventStore manages event imports.
type EventStore interface {
	ImportEvents(ctx context.Context, token, owner, repo string, months int) (map[string]int, *ImportSummary, error)
	UpdateEvents(ctx context.Context, token string, concurrency int) (map[string]int, error)
}

// InsightsStore provides analytics and insights queries.
type InsightsStore interface {
	GetInsightsSummary(org, repo, entity *string, months int) (*InsightsSummary, error)
	GetDailyActivity(org, repo, entity *string, months int) (*DailyActivitySeries, error)
	GetContributorRetention(org, repo, entity *string, months int) (*RetentionSeries, error)
	GetPRReviewRatio(org, repo, entity *string, months int) (*PRReviewRatioSeries, error)
	GetChangeFailureRate(org, repo, entity *string, months int) (*ChangeFailureRateSeries, error)
	GetReviewLatency(org, repo, entity *string, months int) (*ReviewLatencySeries, error)
	GetTimeToMerge(org, repo, entity *string, months int) (*VelocitySeries, error)
	GetTimeToClose(org, repo, entity *string, months int) (*VelocitySeries, error)
	GetTimeToRestoreBugs(org, repo, entity *string, months int) (*VelocitySeries, error)
	GetPRSizeDistribution(org, repo, entity *string, months int) (*PRSizeSeries, error)
	GetForksAndActivity(org, repo, entity *string, months int) (*ForksAndActivitySeries, error)
	GetContributorFunnel(org, repo, entity *string, months int) (*ContributorFunnelSeries, error)
	GetContributorMomentum(org, repo, entity *string, months int) (*MomentumSeries, error)
	GetContributorProfile(username string, org, repo, entity *string, months int) (*ContributorProfileSeries, error)
}

// ReleaseStore manages release imports and queries.
type ReleaseStore interface {
	ImportReleases(ctx context.Context, token, owner, repo string) error
	ImportAllReleases(ctx context.Context, token string) error
	GetReleaseCadence(org, repo, entity *string, months int) (*ReleaseCadenceSeries, error)
	GetReleaseDownloads(org, repo *string, months int) (*ReleaseDownloadsSeries, error)
	GetReleaseDownloadsByTag(org, repo *string, months int) (*ReleaseDownloadsByTagSeries, error)
}

// ContainerStore manages container version imports and queries.
type ContainerStore interface {
	ImportContainerVersions(ctx context.Context, token, org, repo string) error
	ImportAllContainerVersions(ctx context.Context, token string) error
	GetContainerActivity(org, repo *string, months int) (*ContainerActivitySeries, error)
}

// RepoMetaStore manages repository metadata imports and queries.
type RepoMetaStore interface {
	ImportRepoMeta(ctx context.Context, token, owner, repo string) error
	ImportAllRepoMeta(ctx context.Context, token string) error
	GetRepoMetas(org, repo *string) ([]*RepoMeta, error)
}

// MetricHistoryStore manages repository metric history imports and queries.
type MetricHistoryStore interface {
	ImportRepoMetricHistory(ctx context.Context, token, owner, repo string) error
	ImportAllRepoMetricHistory(ctx context.Context, token string) error
	GetRepoMetricHistory(org, repo *string) ([]*RepoMetricHistory, error)
}

// ReputationStore manages reputation scoring.
type ReputationStore interface {
	ImportReputation(org, repo *string) (*ReputationResult, error)
	ImportDeepReputation(ctx context.Context, token string, limit int, org, repo *string) (*DeepReputationResult, error)
	GetOrComputeDeepReputation(ctx context.Context, token, username string) (*UserReputation, error)
	ComputeDeepReputation(ctx context.Context, token, username string) (*UserReputation, error)
	GetReputationDistribution(org, repo, entity *string, months int) (*ReputationDistribution, error)
}

// Store is the top-level interface composing all sub-interfaces.
type Store interface {
	io.Closer
	StateStore
	DeleteStore
	SubstitutionStore
	EntityStore
	RepoStore
	OrgStore
	DeveloperStore
	QueryStore
	EventStore
	InsightsStore
	ReleaseStore
	ContainerStore
	RepoMetaStore
	MetricHistoryStore
	ReputationStore
}
