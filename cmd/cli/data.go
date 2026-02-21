package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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

// queryHandler is used by the search bar
func queryHandler(c *gin.Context) {
	q := c.Query("q")
	v := c.Query("v")

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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error querying org like data",
		})
		return
	}

	c.JSON(http.StatusOK, items)
}

func mapCountedItemsToSeries(res []*data.CountedItem) *SeriesData[int] {
	slog.Debug("items", "count", len(res))

	// trim
	if len(res) > percentageListLimit {
		res = res[:percentageListLimit]
	}

	sum := 0
	data := &SeriesData[int]{
		Labels: make([]string, 0),
		Data:   make([]int, 0),
	}
	for _, v := range res {
		sum += v.Count
		data.Labels = append(data.Labels, v.Name)
		data.Data = append(data.Data, v.Count)
	}

	if sum < hundredPercent {
		data.Labels = append(data.Labels, categoryOther)
		data.Data = append(data.Data, hundredPercent-sum)
	}
	return data
}

func developerDataHandler(c *gin.Context) {
	percentageHandler(c, data.GetDeveloperPercentages)
}

func entityDataHandler(c *gin.Context) {
	percentageHandler(c, data.GetEntityPercentages)
}

type percentageProvider func(db *sql.DB, entity, org, repo *string, ex []string, months int) ([]*data.CountedItem, error)

func percentageHandler(c *gin.Context, fn percentageProvider) {
	months := queryAsInt(c, "m", data.EventAgeMonthsDefault)
	org := c.Query("o")
	repo := c.Query("r")
	entity := c.Query("e")
	exclude := strings.Split(c.Query("x"), arraySelector)

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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error querying event type series",
		})
		return
	}

	data := mapCountedItemsToSeries(res)

	c.JSON(http.StatusOK, data)
}

func eventDataHandler(c *gin.Context) {
	months := queryAsInt(c, "m", data.EventAgeMonthsDefault)
	org := c.Query("o")
	repo := c.Query("r")
	entity := c.Query("e")

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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error querying event type series",
		})
		return
	}

	c.JSON(http.StatusOK, res)
}

func queryAsInt(c *gin.Context, key string, def int) int {
	v := c.Query(key)
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

func eventSearchHandler(c *gin.Context) {
	var q data.EventSearchCriteria
	if err := c.ShouldBindJSON(&q); err != nil {
		slog.Error("error binding json", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "error binding json",
		})
		return
	}

	if org, repo, ok := parseRepo(q.Repo); ok {
		q.Org = org
		q.Repo = repo
	}

	// hack to reverse the chart label formatting
	// TODO: fix this in the frontend
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error querying event type series",
		})
		return
	}

	c.JSON(http.StatusOK, res)
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
