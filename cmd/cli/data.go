package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/mchmarny/dctl/pkg/data"
)

const (
	percentageListLimit = 9
	repoNamePartsLimit  = 2
	hundredPercent      = 100
	categoryOther       = "ALL OTHERS"
	arraySelector       = "|"
)

type SeriesData[T any] struct {
	Labels []string `json:"labels"`
	Data   []T      `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func queryAPIHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	v := r.URL.Query().Get("v")

	db := getDBOrFail()
	defer db.Close()

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

func developerDataAPIHandler(w http.ResponseWriter, r *http.Request) {
	percentageAPIHandler(w, r, data.GetDeveloperPercentages)
}

func entityDataAPIHandler(w http.ResponseWriter, r *http.Request) {
	percentageAPIHandler(w, r, data.GetEntityPercentages)
}

type percentageProvider func(db *sql.DB, entity, org, repo *string, ex []string, months int) ([]*data.CountedItem, error)

func percentageAPIHandler(w http.ResponseWriter, r *http.Request, fn percentageProvider) {
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

	db := getDBOrFail()
	defer db.Close()

	res, err := fn(db, optional(entity), optional(org), optional(repo), exclude, months)
	if err != nil {
		slog.Error("failed to get event type series", "error", err)
		writeError(w, http.StatusInternalServerError, "error querying event type series")
		return
	}

	writeJSON(w, http.StatusOK, mapCountedItemsToSeries(res))
}

func eventDataAPIHandler(w http.ResponseWriter, r *http.Request) {
	months := queryParamInt(r, "m", data.EventAgeMonthsDefault)
	org := r.URL.Query().Get("o")
	repo := r.URL.Query().Get("r")
	entity := r.URL.Query().Get("e")

	slog.Debug("event type query", "org", org, "repo", repo, "entity", entity, "months", months)

	if orgStr, repoStr, ok := parseRepo(&repo); ok {
		org = *orgStr
		repo = *repoStr
	}

	db := getDBOrFail()
	defer db.Close()

	res, err := data.GetEventTypeSeries(db, optional(org), optional(repo), optional(entity), months)
	if err != nil {
		slog.Error("failed to get event type series", "error", err)
		writeError(w, http.StatusInternalServerError, "error querying event type series")
		return
	}

	writeJSON(w, http.StatusOK, res)
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

	return i
}

func eventSearchAPIHandler(w http.ResponseWriter, r *http.Request) {
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

	db := getDBOrFail()
	defer db.Close()

	res, err := data.SearchEvents(db, &q)
	if err != nil {
		slog.Error("failed to execute event search", "error", err)
		writeError(w, http.StatusInternalServerError, "error querying event type series")
		return
	}

	writeJSON(w, http.StatusOK, res)
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
