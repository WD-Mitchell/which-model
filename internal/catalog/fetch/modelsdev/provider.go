package modelsdev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// ProvidersURL is the models.dev providers endpoint. It is the exact URL
// pinned by the F08 tests and the allow-list entry for every request.
const ProvidersURL = "https://models.dev/api.json"

// ProviderModel is one non-deprecated models.dev provider record.
type ProviderModel struct {
	Provider  string
	ModelID   string
	Name      string
	Status    string
	BaseModel string
	// Reasoning reports whether the record exposes effort levels
	// (len(EffortLevels) > 0 after normalization).
	Reasoning bool
	// EffortLevels are the record's reasoning effort levels: parsed via
	// identity.ParseEffort, normalized (invalid levels such as "none" or
	// "default" become "default", then identity.CollapseReasoning), sorted,
	// deduplicated. Levels that are not a subset of identity.ReasoningLevels
	// are a hard error.
	EffortLevels []string
}

// providerRecord mirrors one element of the models.dev providers array.
type providerRecord struct {
	Provider         string `json:"provider"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	BaseModel        string `json:"base_model"`
	ReasoningOptions *struct {
		Values []string `json:"values"`
	} `json:"reasoning_options"`
}

// FetchModelsDevProvidersFrom fetches and maps the models.dev providers
// endpoint at url. Records with status "deprecated" are dropped.
func FetchModelsDevProvidersFrom(client *httpkit.Client, url string) ([]ProviderModel, error) {
	client.SetAllowList([]string{url})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, &fetch.Error{Code: "network", Err: err}
	}
	req.Header.Set("Accept", "application/json")
	body, err := client.Do(context.Background(), req)
	if err != nil {
		return nil, wrapHTTPError(err)
	}

	var records []providerRecord
	if err := json.Unmarshal(body, &records); err != nil {
		return nil, wrapUnmarshalError(err)
	}

	models := make([]ProviderModel, 0, len(records))
	for _, rec := range records {
		if rec.Status == "deprecated" {
			continue
		}
		m := ProviderModel{
			Provider:  rec.Provider,
			ModelID:   rec.ID,
			Name:      identity.CleanModelName(rec.Name),
			Status:    rec.Status,
			BaseModel: rec.BaseModel,
		}
		if rec.ReasoningOptions != nil {
			efforts, err := normalizeEfforts(rec.ReasoningOptions.Values)
			if err != nil {
				return nil, err
			}
			m.EffortLevels = efforts
			m.Reasoning = len(efforts) > 0
		}
		models = append(models, m)
	}
	return models, nil
}

// FetchModelsDevProviders is the production wrapper using the pinned
// ProvidersURL and a default httpkit client.
func FetchModelsDevProviders() ([]ProviderModel, error) {
	return FetchModelsDevProvidersFrom(httpkit.NewClient(), ProvidersURL)
}

// normalizeEfforts maps raw reasoning_options.values to canonical effort
// levels. "none" and "default" are not valid levels but are accepted and
// normalized to "default", which identity.CollapseReasoning then collapses
// to "high". Any other value that is not a subset of identity.ReasoningLevels
// is a hard error.
func normalizeEfforts(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	levels := make([]string, 0, len(values))
	for _, v := range values {
		level, ok := identity.ParseEffort(v)
		if !ok {
			switch strings.ToLower(strings.TrimSpace(v)) {
			case "none", "default":
				level = "default"
			default:
				return nil, unsupportedError("unsupported reasoning effort %q", v)
			}
		}
		level = identity.CollapseReasoning(level)
		if !seen[level] {
			seen[level] = true
			levels = append(levels, level)
		}
	}
	sort.Strings(levels)
	return levels, nil
}

// wrapHTTPError maps an *httpkit.Error to its global Failure.Code.
func wrapHTTPError(err error) *fetch.Error {
	var he *httpkit.Error
	if errors.As(err, &he) {
		switch he.StatusCode {
		case http.StatusUnauthorized:
			return &fetch.Error{Code: "unauthorized", Err: err}
		case http.StatusForbidden:
			return &fetch.Error{Code: "access_denied", Err: err}
		case http.StatusTooManyRequests:
			return &fetch.Error{Code: "rate_limited", Err: err}
		}
		if he.StatusCode >= 500 {
			return &fetch.Error{Code: "provider_status", Err: err}
		}
		return &fetch.Error{Code: he.Code, Err: err}
	}
	return &fetch.Error{Code: "network", Err: err}
}

// wrapUnmarshalError classifies a JSON decode failure: syntax problems are
// response_json; shape/type problems are unsupported_response.
func wrapUnmarshalError(err error) *fetch.Error {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) {
		return &fetch.Error{Code: "unsupported_response", Err: err}
	}
	return &fetch.Error{Code: "response_json", Err: err}
}

func unsupportedError(format string, args ...any) *fetch.Error {
	return &fetch.Error{Code: "unsupported_response", Err: fmt.Errorf(format, args...)}
}
