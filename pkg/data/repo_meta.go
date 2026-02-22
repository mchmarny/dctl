package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/mchmarny/dctl/pkg/net"
)

const (
	upsertRepoMetaSQL = `INSERT INTO repo_meta (org, repo, stars, forks, open_issues, language, license, archived, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(org, repo) DO UPDATE SET
			stars = ?, forks = ?, open_issues = ?, language = ?, license = ?, archived = ?, updated_at = ?
	`

	selectRepoMetaSQL = `SELECT org, repo, stars, forks, open_issues, language, license, archived, updated_at
		FROM repo_meta
		WHERE org = COALESCE(?, org)
		  AND repo = COALESCE(?, repo)
		ORDER BY org, repo
	`
)

type RepoMeta struct {
	Org        string `json:"org" yaml:"org"`
	Repo       string `json:"repo" yaml:"repo"`
	Stars      int    `json:"stars" yaml:"stars"`
	Forks      int    `json:"forks" yaml:"forks"`
	OpenIssues int    `json:"open_issues" yaml:"openIssues"`
	Language   string `json:"language" yaml:"language"`
	License    string `json:"license" yaml:"license"`
	Archived   bool   `json:"archived" yaml:"archived"`
	UpdatedAt  string `json:"updated_at" yaml:"updatedAt"`
}

func ImportRepoMeta(dbPath, token, owner, repo string) error {
	ctx := context.Background()
	client := github.NewClient(net.GetOAuthClient(ctx, token))

	r, resp, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error getting repo %s/%s: %w", owner, repo, err)
	}
	checkRateLimit(resp)

	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %w", err)
	}
	defer db.Close()

	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	lang := r.GetLanguage()
	var license string
	if r.License != nil {
		license = r.License.GetSPDXID()
	}
	archived := 0
	if r.GetArchived() {
		archived = 1
	}

	_, err = db.Exec(upsertRepoMetaSQL,
		owner, repo, r.GetStargazersCount(), r.GetForksCount(), r.GetOpenIssuesCount(),
		lang, license, archived, now,
		r.GetStargazersCount(), r.GetForksCount(), r.GetOpenIssuesCount(),
		lang, license, archived, now,
	)
	if err != nil {
		return fmt.Errorf("error upserting repo meta %s/%s: %w", owner, repo, err)
	}

	slog.Debug("imported repo metadata", "org", owner, "repo", repo)
	return nil
}

func ImportAllRepoMeta(dbPath, token string) error {
	db, err := GetDB(dbPath)
	if err != nil {
		return fmt.Errorf("error getting DB: %w", err)
	}
	defer db.Close()

	list, err := GetAllOrgRepos(db)
	if err != nil {
		return fmt.Errorf("error getting org/repo list: %w", err)
	}

	for _, r := range list {
		if err := ImportRepoMeta(dbPath, token, r.Org, r.Repo); err != nil {
			slog.Error("error importing repo metadata", "org", r.Org, "repo", r.Repo, "error", err)
		}
	}

	return nil
}

func GetRepoMetas(db *sql.DB, org, repo *string) ([]*RepoMeta, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	rows, err := db.Query(selectRepoMetaSQL, org, repo)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to query repo meta: %w", err)
	}
	defer rows.Close()

	list := make([]*RepoMeta, 0)
	for rows.Next() {
		m := &RepoMeta{}
		var archived int
		if err := rows.Scan(&m.Org, &m.Repo, &m.Stars, &m.Forks, &m.OpenIssues,
			&m.Language, &m.License, &archived, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan repo meta row: %w", err)
		}
		m.Archived = archived != 0
		list = append(list, m)
	}

	return list, nil
}
