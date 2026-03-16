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
	"sync"

	"github.com/mchmarny/devpulse/pkg/net"
)

const (
	affilFileURL = "https://raw.githubusercontent.com/cncf/gitdm/master/developers_affiliations%d.txt"
)

// UpdateDevelopersWithCNCFEntityAffiliations updates the developers with the CNCF entity affiliations.
func UpdateDevelopersWithCNCFEntityAffiliations(ctx context.Context, db *sql.DB, client *http.Client) (*AffiliationImportResult, error) {
	if db == nil {
		return nil, ErrDBNotInitialized
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

	const maxConcurrent = 10

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		sem      = make(chan struct{}, maxConcurrent)
		merged   []*Developer
		firstErr error
	)

	for _, u := range dbDevs {
		dev, ok := cncfDevs[u]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(username string, cDev *CNCFDeveloper) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			d, mergeErr := MergeDeveloper(ctx, db, client, username, cDev)
			mu.Lock()
			defer mu.Unlock()
			if mergeErr != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("error updating developer %s: %w", username, mergeErr)
				}
				return
			}
			if d != nil {
				merged = append(merged, d)
			}
		}(u, dev)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Batch-save all merged developers in a single transaction.
	if len(merged) > 0 {
		if err := SaveDevelopers(db, merged); err != nil {
			return nil, fmt.Errorf("saving merged developers: %w", err)
		}
	}

	res.MappedDevs = len(merged)

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
		ok, err := loadAffiliations(ctx, url, devs)
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

func loadAffiliations(ctx context.Context, url string, devs map[string]*CNCFDeveloper) (bool, error) {
	if url == "" {
		return false, errors.New("url is empty")
	}

	f, err := os.CreateTemp("", "affils")
	if err != nil {
		return false, fmt.Errorf("error creating temp file: %w", err)
	}

	path := f.Name()
	slog.Debug("downloading", "url", url, "path", path)
	if err = net.Download(ctx, url, path); err != nil {
		if errors.Is(err, net.ErrURLNotFound) {
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

	f, err := os.Open(path) //nolint:gosec,nolintlint // G703: path from os.CreateTemp
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
			parts := strings.SplitN(line, ":", 2)
			p = &CNCFDeveloper{
				Username:     parts[0],
				Identities:   make([]string, 0),
				Affiliations: make([]*CNCFAffiliation, 0),
			}

			if len(parts) > 1 {
				addresses := strings.Split(parts[1], ",")
				for _, address := range addresses {
					if strings.Contains(address, "users.noreply.github.com") {
						continue
					}
					p.Identities = append(p.Identities, strings.ReplaceAll(strings.TrimSpace(address), "!", "@"))
				}
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

	// add final developer
	if p != nil {
		devs[p.Username] = p
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %s: %w", path, err)
	}

	return nil
}
