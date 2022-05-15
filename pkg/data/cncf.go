package data

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	affilFileURL              = "https://raw.githubusercontent.com/cncf/gitdm/master/developers_affiliations%d.txt"
	numOfAffilFilesToDownload = 100
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
		return nil, errors.Errorf("client is required")
	}

	dbDevs, err := GetDeveloperUsernames(db)
	if err != nil {
		return nil, errors.Wrap(err, "error getting developers from db")
	}

	cncfDevs, err := GetCNCFEntityAffiliations()
	if err != nil {
		return nil, errors.Wrap(err, "error getting CNCF affiliations")
	}

	start := time.Now()
	res := &AffiliationImportResult{
		DBDevs:   len(dbDevs),
		CNCFDevs: len(cncfDevs),
	}

	for _, u := range dbDevs {
		if dev, ok := cncfDevs[u]; ok {
			if err := UpdateDeveloper(ctx, db, client, u, dev); err != nil {
				return nil, errors.Wrapf(err, "error updating developer %s", u)
			}
			res.MappedDevs++
			continue
		}
		log.Debugf("no CNCF affiliation for %s", u)
	}

	// run this on the end to update the user entities that were not part of the above process
	if err := CleanEntities(db); err != nil {
		return nil, errors.Wrap(err, "error cleaning entities")
	}

	res.Duration = time.Since(start).String()

	return res, nil
}

func GetCNCFEntityAffiliations() (map[string]*CNCFDeveloper, error) {
	start := time.Now()
	urls := make([]string, 0)
	for i := 1; i <= numOfAffilFilesToDownload; i++ {
		urls = append(urls, fmt.Sprintf(affilFileURL, i))
	}

	completed := 0
	devs := make(map[string]*CNCFDeveloper)
	for i, url := range urls {
		ok, err := loadAffiliations(url, devs)
		if err != nil {
			return devs, errors.Wrapf(err, "import affiliation from URS %d - %s", i, url)
		}
		completed++
		if !ok {
			break
		}
	}
	log.Debugf("downloaded and processed %d files in %s", completed, time.Since(start).String())

	return devs, nil
}

func loadAffiliations(url string, devs map[string]*CNCFDeveloper) (bool, error) {
	if url == "" {
		return false, errors.New("url is empty")
	}

	f, err := os.CreateTemp("", "affils")
	if err != nil {
		return false, errors.Wrapf(err, "error creating temp file")
	}

	path := f.Name()
	log.Debugf("downloading %s to %s", url, path)
	if err = net.Download(url, path); err != nil {
		if err == net.ErrorURLNotFound {
			log.Debugf("url not found: %s", url)
			return false, nil // return raw error
		}
		return false, errors.Wrapf(err, "error downloading file: %s", url)
	}
	defer os.Remove(f.Name())

	log.Debugf("extracting %s", path)
	if err := extractAffiliations(path, devs); err != nil {
		return false, errors.Wrapf(err, "error extracting file: %s", path)
	}

	return true, nil
}

func extractAffiliations(path string, devs map[string]*CNCFDeveloper) error {
	if path == "" {
		return fmt.Errorf("path not set")
	}

	f, err := os.Open(path)
	if err != nil {
		return errors.Wrapf(err, "error opening file: %s", path)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	var p *CNCFDeveloper
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return errors.Wrapf(err, "error reading file: %s", path)
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
				p.Identities = append(p.Identities, strings.Replace(strings.TrimSpace(address), "!", "@", -1))
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
		return errors.Wrapf(err, "error reading file: %s", path)
	}

	return nil
}
