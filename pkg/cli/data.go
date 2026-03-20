package cli

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	percentageListLimit       = 9
	repoNamePartsLimit        = 2
	hundredPercent            = 100
	categoryOther             = "ALL OTHERS"
	arraySelector             = "|"
	maxRequestBodyBytes int64 = 1 << 20 // 1 MB
)

type SeriesData[T any] struct {
	Labels []string `json:"labels" yaml:"labels"`
	Data   []T      `json:"data" yaml:"data"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type insightParams struct {
	months int
	org    *string
	repo   *string
}

func parseInsightParams(r *http.Request) insightParams {
	months := queryParamInt(r, "m", data.EventAgeMonthsDefault)
	org := r.URL.Query().Get("o")
	repo := r.URL.Query().Get("r")
	if orgStr, repoStr, ok := parseRepo(optional(repo)); ok {
		org = *orgStr
		repo = *repoStr
	}
	return insightParams{months: months, org: optional(org), repo: optional(repo)}
}

func minDateAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		minDate, err := store.GetMinEventDate(p.org, p.repo)
		if err != nil {
			slog.Error("failed to get min event date", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to get min date")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"min_date": minDate})
	}
}

func queryAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		v := r.URL.Query().Get("v")

		var items []*data.ListItem
		var err error

		const (
			scopeOrg  = "org"
			scopeRepo = "repo"
		)

		switch v {
		case scopeOrg:
			items, err = store.GetOrgLike(q, queryResultLimitDefault)
		case scopeRepo:
			items, err = store.GetRepoLike(q, queryResultLimitDefault)
		case "entity":
			items, err = store.GetEntityLike(q, queryResultLimitDefault)
		case "all":
			half := queryResultLimitDefault / 2
			orgs, orgErr := store.GetOrgLike(q, half)
			if orgErr != nil {
				err = orgErr
				break
			}
			for _, o := range orgs {
				o.Type = scopeOrg
			}
			repos, repoErr := store.GetRepoLike(q, half)
			if repoErr != nil {
				err = repoErr
				break
			}
			for _, r := range repos {
				r.Type = scopeRepo
			}
			items = append(items, orgs...)
			items = append(items, repos...)
		default:
			items = []*data.ListItem{}
		}

		if err != nil {
			slog.Error("failed to get org like data", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying org like data")
			return
		}

		writeJSON(w, http.StatusOK, items)
	}
}

func mapCountedItemsToSeries(res []*data.CountedItem) *SeriesData[int] {
	slog.Debug("items", "count", len(res))

	if len(res) > percentageListLimit {
		res = res[:percentageListLimit]
	}

	sum := 0
	d := &SeriesData[int]{
		Labels: make([]string, 0),
		Data:   make([]int, 0),
	}
	for _, v := range res {
		sum += v.Count
		d.Labels = append(d.Labels, v.Name)
		d.Data = append(d.Data, v.Count)
	}

	if sum < hundredPercent {
		d.Labels = append(d.Labels, categoryOther)
		d.Data = append(d.Data, hundredPercent-sum)
	}
	return d
}

func developerDataAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		percentageAPIHandler(w, r, store.GetDeveloperPercentages)
	}
}

func entityDataAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		percentageAPIHandler(w, r, store.GetEntityPercentages)
	}
}

type percentageProvider func(entity, org, repo *string, ex []string, months int) ([]*data.CountedItem, error)

func percentageAPIHandler(w http.ResponseWriter, r *http.Request, fn percentageProvider) {
	months := queryParamInt(r, "m", data.EventAgeMonthsDefault)
	org := r.URL.Query().Get("o")
	repo := r.URL.Query().Get("r")
	entity := r.URL.Query().Get("e")
	var exclude []string
	if x := r.URL.Query().Get("x"); x != "" {
		exclude = strings.Split(x, arraySelector)
	}

	slog.Debug("event type query", "org", org, "repo", repo, "entity", entity, "months", months)

	if orgStr, repoStr, ok := parseRepo(&repo); ok {
		org = *orgStr
		repo = *repoStr
	}

	res, err := fn(optional(entity), optional(org), optional(repo), exclude, months)
	if err != nil {
		slog.Error("failed to get event type series", "error", err)
		writeError(w, http.StatusInternalServerError, "error querying event type series")
		return
	}

	writeJSON(w, http.StatusOK, mapCountedItemsToSeries(res))
}

func eventDataAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := r.URL.Query().Get("e")
		res, err := store.GetEventTypeSeries(p.org, p.repo, optional(entity), p.months)
		if err != nil {
			slog.Error("failed to get event type series", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying event type series")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func queryParamInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}

	i, err := strconv.Atoi(v)
	if err != nil {
		slog.Error("error converting query string to int", "value", v, "error", err)
		return def
	}

	if i < 1 || i > 120 {
		return def
	}

	return i
}

func eventSearchAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		var q data.EventSearchCriteria
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			slog.Error("error binding json", "error", err)
			writeError(w, http.StatusBadRequest, "error binding json")
			return
		}

		if org, repo, ok := parseRepo(q.Repo); ok {
			q.Org = org
			q.Repo = repo
		}

		if q.Type != nil {
			eType := *q.Type
			switch eType {
			case "PR":
				eType = data.EventTypePR
			case "PR-Review":
				eType = data.EventTypePRReview
			case "Issue":
				eType = data.EventTypeIssue
			case "Issue-Comment":
				eType = data.EventTypeIssueComment
			case "Fork":
				eType = data.EventTypeFork
			default:
				eType = ""
			}
			if eType != "" {
				q.Type = &eType
			}
		}

		slog.Debug("event search query", "query", q)

		res, err := store.SearchEvents(&q)
		if err != nil {
			slog.Error("failed to execute event search", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying event type series")
			return
		}

		writeJSON(w, http.StatusOK, res)
	}
}

func insightsSummaryAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := r.URL.Query().Get("e")
		res, err := store.GetInsightsSummary(p.org, p.repo, optional(entity), p.months)
		if err != nil {
			slog.Error("failed to get insights summary", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying insights summary")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsDailyActivityAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := r.URL.Query().Get("e")
		res, err := store.GetDailyActivity(p.org, p.repo, optional(entity), p.months)
		if err != nil {
			slog.Error("failed to get daily activity", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying daily activity")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsRetentionAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetContributorRetention(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get contributor retention", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying contributor retention")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsPRRatioAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetPRReviewRatio(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get PR review ratio", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying PR review ratio")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func entityDevelopersAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entity := r.URL.Query().Get("e")
		if entity == "" {
			writeError(w, http.StatusBadRequest, "entity parameter required")
			return
		}

		res, err := store.GetEntity(entity)
		if err != nil {
			slog.Error("failed to get entity developers", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying entity developers")
			return
		}

		writeJSON(w, http.StatusOK, res)
	}
}

func insightsRepoMetaAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetRepoMetas(p.org, p.repo)
		if err != nil {
			slog.Error("failed to get repo metadata", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying repo metadata")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsRepoOverviewAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetRepoOverview(p.org, p.months)
		if err != nil {
			slog.Error("failed to get repo overview", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying repo overview")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsRepoMetricHistoryAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetRepoMetricHistory(p.org, p.repo, p.months)
		if err != nil {
			slog.Error("failed to get repo metric history", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying repo metric history")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsReleaseCadenceAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetReleaseCadence(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get release cadence", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying release cadence")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsReleaseDownloadsAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetReleaseDownloads(p.org, p.repo, p.months)
		if err != nil {
			slog.Error("failed to get release downloads", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying release downloads")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsReleaseDownloadsByTagAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetReleaseDownloadsByTag(p.org, p.repo, p.months)
		if err != nil {
			slog.Error("failed to get release downloads by tag", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying release downloads by tag")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsContainerActivityAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetContainerActivity(p.org, p.repo, p.months)
		if err != nil {
			slog.Error("failed to get container activity", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying container activity")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsTimeToMergeAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetTimeToMerge(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get time to merge", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying time to merge")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsTimeToCloseAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetTimeToClose(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get time to close", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying time to close")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsTimeToRestoreAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetTimeToRestoreBugs(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get time to restore", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying time to restore")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsChangeFailureRateAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetChangeFailureRate(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get change failure rate", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying change failure rate")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsReviewLatencyAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetReviewLatency(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get review latency", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying review latency")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsPRSizeAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetPRSizeDistribution(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get PR size distribution", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying PR size distribution")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsContributorMomentumAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetContributorMomentum(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get contributor momentum", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying contributor momentum")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsContributorFunnelAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetContributorFunnel(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get contributor funnel", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying contributor funnel")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsContributorProfileAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		username := r.URL.Query().Get("u")
		if username == "" {
			writeError(w, http.StatusBadRequest, "username parameter (u) is required")
			return
		}
		res, err := store.GetContributorProfile(username, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get contributor profile", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying contributor profile")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func developerSearchAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		q := r.URL.Query().Get("q")
		if q == "" {
			writeError(w, http.StatusBadRequest, "query parameter (q) is required")
			return
		}
		res, err := store.SearchDeveloperUsernames(q, p.org, p.repo, p.months, 10)
		if err != nil {
			slog.Error("failed to search developers", "error", err)
			writeError(w, http.StatusInternalServerError, "error searching developers")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsTimeToFirstResponseAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetTimeToFirstResponse(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get time to first response", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying time to first response")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsIssueRatioAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetIssueOpenCloseRatio(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get issue open/close ratio", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying issue open/close ratio")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsForksAndActivityAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetForksAndActivity(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get forks and activity", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying forks and activity")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsReputationAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := store.GetReputationDistribution(p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get reputation distribution", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying reputation distribution")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsGeneratedAPIHandler(store data.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := store.GetRepoInsights(p.org, p.repo)
		if err != nil {
			slog.Error("failed to get generated insights", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying generated insights")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func parseRepo(repo *string) (*string, *string, bool) {
	if repo == nil {
		return nil, nil, false
	}

	repoParts := strings.Split(*repo, "/")
	if len(repoParts) != repoNamePartsLimit {
		return nil, nil, false
	}

	o := strings.TrimSpace(repoParts[0])
	r := strings.TrimSpace(repoParts[1])

	return &o, &r, true
}
