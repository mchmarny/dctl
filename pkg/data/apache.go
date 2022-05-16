package data

import (
	"database/sql"
	"time"

	"github.com/mchmarny/dctl/pkg/net"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	afDevURL = "http://home.apache.org/public/public_ldap_people.json"
)

type AFDeveloperList struct {
	Count int                    `json:"people_count,omitempty"`
	List  map[string]AFDeveloper `json:"people,omitempty"`
}

type AFDeveloper struct {
	Name string `json:"name,omitempty"`
}

type ApacheUpdateResult struct {
	Duration   string `json:"duration,omitempty"`
	DBDevs     int    `json:"db_devs,omitempty"`
	AFDevs     int    `json:"af_devs,omitempty"`
	MappedDevs int    `json:"mapped_devs,omitempty"`
}

func UpdateNoFullnameDevelopersFromApache(db *sql.DB) (*ApacheUpdateResult, error) {
	if db == nil {
		return nil, errDBNotInitialized
	}

	var afList AFDeveloperList
	if err := net.GetJSON(afDevURL, &afList); err != nil {
		return nil, errors.Wrap(err, "failed to get list of developers")
	}

	if len(afList.List) == 0 {
		return nil, errors.New("no Apache Foundation developers found")
	}

	dbList, err := GetNoFullnameDeveloperUsernames(db)
	if err != nil {
		return nil, errors.Wrap(err, "error getting developers from db")
	}

	if len(dbList) == 0 {
		return nil, errors.New("no developers found in db")
	}

	dbDevs := make(map[string]bool)
	for _, dev := range dbList {
		dbDevs[dev] = true
	}

	start := time.Now()
	res := &ApacheUpdateResult{
		DBDevs: len(dbDevs),
		AFDevs: len(afList.List),
	}

	foundDevs := make(map[string]string)
	for u, d := range afList.List {
		if _, ok := dbDevs[u]; ok {
			foundDevs[u] = d.Name
			res.MappedDevs++
			continue
		}
		log.Debugf("not record for %s found in Apache Foundation", u)
	}

	if err := UpdateDeveloperNames(db, foundDevs); err != nil {
		return nil, errors.Wrap(err, "error updating developer names")
	}

	res.Duration = time.Since(start).String()

	return res, nil
}
