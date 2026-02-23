package cli

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/mchmarny/devpulse/pkg/data"
)

const (
	percentageListLimit = 9
	repoNamePartsLimit  = 2
	hundredPercent      = 100
	categoryOther       = "ALL OTHERS"
	arraySelector       = "|"
)

type SeriesData[T any] struct {
	Labels []string `json:"labels" yaml:"labels"`
	Data   []T      `json:"data" yaml:"data"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
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

func minDateAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		minDate, err := data.GetMinEventDate(db, p.org, p.repo)
		if err != nil {
			slog.Error("failed to get min event date", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to get min date")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"min_date": minDate})
	}
}

func queryAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		v := r.URL.Query().Get("v")

		var items []*data.ListItem
		var err error

		switch v {
		case "org":
			items, err = data.GetOrgLike(db, q, queryResultLimitDefault)
		case "repo":
			items, err = data.GetRepoLike(db, q, queryResultLimitDefault)
		case "entity":
			items, err = data.GetEntityLike(db, q, queryResultLimitDefault)
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

func developerDataAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		percentageAPIHandler(w, r, db, data.GetDeveloperPercentages)
	}
}

func entityDataAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		percentageAPIHandler(w, r, db, data.GetEntityPercentages)
	}
}

type percentageProvider func(db *sql.DB, entity, org, repo *string, ex []string, months int) ([]*data.CountedItem, error)

func percentageAPIHandler(w http.ResponseWriter, r *http.Request, db *sql.DB, fn percentageProvider) {
	months := queryParamInt(r, "m", data.EventAgeMonthsDefault)
	org := r.URL.Query().Get("o")
	repo := r.URL.Query().Get("r")
	entity := r.URL.Query().Get("e")
	exclude := strings.Split(r.URL.Query().Get("x"), arraySelector)

	slog.Debug("event type query", "org", org, "repo", repo, "entity", entity, "months", months)

	if orgStr, repoStr, ok := parseRepo(&repo); ok {
		org = *orgStr
		repo = *repoStr
	}

	res, err := fn(db, optional(entity), optional(org), optional(repo), exclude, months)
	if err != nil {
		slog.Error("failed to get event type series", "error", err)
		writeError(w, http.StatusInternalServerError, "error querying event type series")
		return
	}

	writeJSON(w, http.StatusOK, mapCountedItemsToSeries(res))
}

func eventDataAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := r.URL.Query().Get("e")
		res, err := data.GetEventTypeSeries(db, p.org, p.repo, optional(entity), p.months)
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

func eventSearchAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		res, err := data.SearchEvents(db, &q)
		if err != nil {
			slog.Error("failed to execute event search", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying event type series")
			return
		}

		writeJSON(w, http.StatusOK, res)
	}
}

func insightsSummaryAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := r.URL.Query().Get("e")
		res, err := data.GetInsightsSummary(db, p.org, p.repo, optional(entity), p.months)
		if err != nil {
			slog.Error("failed to get insights summary", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying insights summary")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsRetentionAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := data.GetContributorRetention(db, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get contributor retention", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying contributor retention")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsPRRatioAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := data.GetPRReviewRatio(db, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get PR review ratio", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying PR review ratio")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func entityDevelopersAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entity := r.URL.Query().Get("e")
		if entity == "" {
			writeError(w, http.StatusBadRequest, "entity parameter required")
			return
		}

		res, err := data.GetEntity(db, entity)
		if err != nil {
			slog.Error("failed to get entity developers", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying entity developers")
			return
		}

		writeJSON(w, http.StatusOK, res)
	}
}

func insightsRepoMetaAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := data.GetRepoMetas(db, p.org, p.repo)
		if err != nil {
			slog.Error("failed to get repo metadata", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying repo metadata")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsReleaseCadenceAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		res, err := data.GetReleaseCadence(db, p.org, p.repo, p.months)
		if err != nil {
			slog.Error("failed to get release cadence", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying release cadence")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsTimeToMergeAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := data.GetTimeToMerge(db, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get time to merge", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying time to merge")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsTimeToCloseAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := data.GetTimeToClose(db, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get time to close", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying time to close")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsForksAndActivityAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := data.GetForksAndActivity(db, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get forks and activity", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying forks and activity")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func insightsReputationAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := parseInsightParams(r)
		entity := optional(r.URL.Query().Get("e"))
		res, err := data.GetReputationDistribution(db, p.org, p.repo, entity, p.months)
		if err != nil {
			slog.Error("failed to get reputation distribution", "error", err)
			writeError(w, http.StatusInternalServerError, "error querying reputation distribution")
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func reputationUserAPIHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("u")
		if username == "" {
			writeError(w, http.StatusBadRequest, "username parameter required")
			return
		}

		token, err := getGitHubToken()
		if err != nil || token == "" {
			slog.Error("failed to get GitHub token for deep score", "error", err)
			writeError(w, http.StatusInternalServerError, "GitHub token not available")
			return
		}

		res, err := data.GetOrComputeDeepReputation(db, token, username)
		if err != nil {
			slog.Error("failed to compute deep reputation", "username", username, "error", err)
			writeError(w, http.StatusInternalServerError, "error computing reputation")
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
