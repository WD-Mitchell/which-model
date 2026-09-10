//go:build !nousage

package cache

import (
	"encoding/json"
	"errors"
	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/privacy"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"regexp"
	"time"
)

var readCompanyPolicy = company.Load
var companyProviderID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)

func companyController() (privacy.Controller, error) {
	p, err := readCompanyPolicy()
	return privacy.Controller{Policy: p}, err
}
func (s *Store) companyWrite(c privacy.Controller, id string, snap usage.Snapshot) error {
	if !companyProviderID.MatchString(id) {
		return errors.New("invalid cache provider")
	}
	if snap.Failure != nil && snap.Failure.Code != "" {
		return errors.New("refusing to cache a failed snapshot")
	}
	data, err := json.Marshal(cacheFile{Snapshot: snap})
	if err != nil {
		return &privacy.Error{Category: privacy.Usage, Operation: "encode"}
	}
	return c.WriteUsage(s.filePath(id), data)
}
func (s *Store) companyRead(c privacy.Controller, id string, ttl time.Duration) (usage.Snapshot, bool, error) {
	if !companyProviderID.MatchString(id) {
		return usage.Snapshot{}, false, errors.New("invalid cache provider")
	}
	data, err := c.Read(s.filePath(id), privacy.Usage)
	if err != nil {
		return usage.Snapshot{}, false, err
	}
	if len(data) == 0 || ttl <= 0 {
		return usage.Snapshot{}, false, ErrCacheMiss
	}
	var record cacheFile
	if json.Unmarshal(data, &record) != nil {
		return usage.Snapshot{}, false, ErrCacheMiss
	}
	return record.Snapshot, time.Since(record.FetchedAt) > ttl, nil
}
