package data

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"log/slog"

	"github.com/mchmarny/dctl/pkg/net"
)

const (
	affilFileURL = "https://raw.githubusercontent.com/cncf/gitdm/master/developers_affiliations%d.txt"
)

type CNCFDeveloper struct {
	Username     string             `json:"username,omitempty"`
	Identities   []string           `json:"identities,omitempty"`
	Affiliations []*CNCFAffiliation `json:"affiliations,omitempty"`
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
	Entity string `json:"entity,omitempty"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
}

type AffiliationImportResult struct {
	Duration   string `json:"duration,omitempty"`
	DBDevs     int    `json:"db_devs,omitempty"`
	CNCFDevs   int    `json:"cncf_devs,omitempty"`
	MappedDevs int    `json:"mapped_devs,omitempty"`
}

// UpdateDevelopersWithCNCFEntityAffiliations updates the developers with the CNCF entity affiliations.
func UpdateDevelopersWithCNCFEntityAffiliations(ctx context.Context, db *sql.DB, client *http.Client) (*AffiliationImportResult, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	if client == nil {
		return nil, fmt.Errorf("client is required")
	}

	dbDevs, err := GetDeveloperUsernames(db)
	if err != nil {
		return nil, fmt.Errorf("error getting developers from db: %w", err)
	}

	cncfDevs, err := GetCNCFEntityAffiliations(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting CNCF affiliations: %w", err)
	}

	start := time.Now()
	res := &AffiliationImportResult{
		DBDevs:   len(dbDevs),
		CNCFDevs: len(cncfDevs),
	}

	for _, u := range dbDevs {
		if dev, ok := cncfDevs[u]; ok {
			if err := UpdateDeveloper(ctx, db, client, u, dev); err != nil {
				return nil, fmt.Errorf("error updating developer %s: %w", u, err)
			}
			res.MappedDevs++
			continue
		}
	}

	// run this on the end to update the user entities that were not part of the above process
	if err := CleanEntities(db); err != nil {
		return nil, fmt.Errorf("error cleaning entities: %w", err)
	}

	res.Duration = time.Since(start).String()

	return res, nil
}

func GetCNCFEntityAffiliations(ctx context.Context) (map[string]*CNCFDeveloper, error) {
	start := time.Now()
	devs := make(map[string]*CNCFDeveloper)
	completed := 0

	for i := 1; ; i++ {
		select {
		case <-ctx.Done():
			return devs, ctx.Err()
		default:
		}

		url := fmt.Sprintf(affilFileURL, i)
		ok, err := loadAffiliations(url, devs)
		if err != nil {
			return devs, fmt.Errorf("loading affiliation file %d (%s): %w", i, url, err)
		}
		if !ok {
			break
		}
		completed++
	}

	slog.Debug("CNCF affiliations loaded",
		"files", completed,
		"developers", len(devs),
		"duration", time.Since(start).String(),
	)

	return devs, nil
}

func loadAffiliations(url string, devs map[string]*CNCFDeveloper) (bool, error) {
	if url == "" {
		return false, errors.New("url is empty")
	}

	f, err := os.CreateTemp("", "affils")
	if err != nil {
		return false, fmt.Errorf("error creating temp file: %w", err)
	}

	path := f.Name()
	slog.Debug("downloading", "url", url, "path", path)
	if err = net.Download(url, path); err != nil {
		if errors.Is(err, net.ErrorURLNotFound) {
			slog.Debug("url not found", "url", url)
			return false, nil // return raw error
		}
		return false, fmt.Errorf("error downloading file: %s: %w", url, err)
	}
	defer os.Remove(f.Name())

	slog.Debug("extracting", "path", path)
	if err := extractAffiliations(path, devs); err != nil {
		return false, fmt.Errorf("error extracting file: %s: %w", path, err)
	}

	return true, nil
}

func extractAffiliations(path string, devs map[string]*CNCFDeveloper) error {
	if path == "" {
		return fmt.Errorf("path not set")
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening file: %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var p *CNCFDeveloper
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading file: %s: %w", path, err)
		}

		// skip empty and comment lines
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		if strings.Contains(line, ":") {
			// finish previous user if any
			if p != nil {
				devs[p.Username] = p
			}

			// create new user
			p = &CNCFDeveloper{
				Username:     strings.Split(line, ":")[0],
				Identities:   make([]string, 0),
				Affiliations: make([]*CNCFAffiliation, 0),
			}

			addressStr := strings.Split(line, ":")[1]
			addresses := strings.Split(addressStr, ",")
			for _, address := range addresses {
				if strings.Contains(address, "users.noreply.github.com") {
					continue
				}
				p.Identities = append(p.Identities, strings.ReplaceAll(strings.TrimSpace(address), "!", "@"))
			}

			continue
		}

		// parse affiliations
		f := &CNCFAffiliation{}

		var nextFrom, nextUntil bool
		idNameParts := make([]string, 0)
		for _, part := range strings.Split(line, " ") {
			if nextFrom {
				f.From = part
				nextFrom = false
				continue
			}

			if nextUntil {
				f.To = part
				nextUntil = false
				continue
			}

			if part == "from" {
				nextFrom = true
				continue
			}

			if part == "until" {
				nextUntil = true
				continue
			}
			idNameParts = append(idNameParts, part)
		}

		// add affiliation identity
		f.Entity = strings.TrimSpace(strings.Join(idNameParts, " "))

		// add affiliation to the current user
		p.Affiliations = append(p.Affiliations, f)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %s: %w", path, err)
	}

	return nil
}
