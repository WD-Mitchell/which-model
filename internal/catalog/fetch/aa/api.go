package aa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	sdecimal "github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// PrimaryURL / FreeURL: the v2 language-models endpoint and its free tier.
// FreeURL is used ONLY as the single 403 fallback.
const (
	PrimaryURL = "https://artificialanalysis.ai/api/v2/language/models"
	FreeURL    = PrimaryURL + "/free"
)

// MaxPages caps the pagination loop (pagination.page must equal the
// requested page on every response, else hard error).
const MaxPages = 100

// AAModel is one deduplicated AA v2 model record.
type AAModel struct {
	Slug                  string // item["slug"]
	IntelligenceIndex     *sdecimal.Decimal
	CodingIndex           *sdecimal.Decimal
	AgenticIndex          *sdecimal.Decimal
	MedianResponseSeconds *sdecimal.Decimal
	CostPerTaskUSD        *sdecimal.Decimal
	Benchmarks            map[string]sdecimal.Decimal // keyed by AABenchmarkField.Column
}

// aaItem mirrors one envelope data element (docs/plan/research/
// model-data-pipeline-spec.md §1.3).
type aaItem struct {
	Slug        string                     `json:"slug"`
	Evaluations map[string]json.RawMessage `json:"evaluations"`
	Performance struct {
		MedianResponseSeconds *sdecimal.Decimal `json:"median_end_to_end_response_time_seconds"`
	} `json:"performance"`
	Cost struct {
		CostPerTask struct {
			TotalCost *sdecimal.Decimal `json:"total_cost"`
		} `json:"cost_per_task"`
	} `json:"artificial_analysis_intelligence_index_cost"`
}

// aaEnvelope is the paginated response envelope.
type aaEnvelope struct {
	Data []json.RawMessage `json:"data"`
	Pagination struct {
		Page       int  `json:"page"`
		HasMore    bool `json:"has_more"`
		TotalPages int  `json:"total_pages"`
	} `json:"pagination"`
}

// mergedModel accumulates max-wins across a slug's tau-variant records.
type mergedModel struct {
	Slug                  string
	IntelligenceIndex     *sdecimal.Decimal
	CodingIndex           *sdecimal.Decimal
	AgenticIndex          *sdecimal.Decimal
	MedianResponseSeconds *sdecimal.Decimal
	CostPerTaskUSD        *sdecimal.Decimal
	Benchmarks            map[string]sdecimal.Decimal
}

// FetchAAv2From fetches the full AA v2 model list from primaryURL, paginated
// ?page=1..N while has_more, capped at MaxPages. On a primary HTTP 403 (any
// page) the entire pagination is retried once on freeURL; every other error
// propagates.
func FetchAAv2From(client *httpkit.Client, apiKey string, primaryURL, freeURL string) ([]AAModel, error) {
	models, err := fetchAllFrom(client, apiKey, primaryURL)
	if err != nil {
		var he *httpkit.Error
		if errors.As(err, &he) && he.StatusCode == http.StatusForbidden {
			return fetchAllFrom(client, apiKey, freeURL)
		}
		return nil, err
	}
	return models, nil
}

// FetchAAv2 is the F23-facing wrapper over the pinned PrimaryURL/FreeURL.
func FetchAAv2(client *httpkit.Client, apiKey string) ([]AAModel, error) {
	return FetchAAv2From(client, apiKey, PrimaryURL, FreeURL)
}

// fetchAllFrom runs one paginated fetch from a single base URL.
func fetchAllFrom(client *httpkit.Client, apiKey, url string) ([]AAModel, error) {
	merged := make(map[string]*mergedModel)
	page := 1
	for {
		if page > MaxPages {
			return nil, unsupported("pagination exceeded %d pages", MaxPages)
		}
		pageURL := url + "?page=" + strconv.Itoa(page)
		client.SetAllowList([]string{pageURL})
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, &fetch.Error{Code: "network", Err: err}
		}
		req.Header.Set("x-api-key", apiKey)
		body, err := client.Do(context.Background(), req)
		if err != nil {
			return nil, wrapHTTPError(err)
		}

		var env aaEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, wrapUnmarshalError(err)
		}
		if env.Pagination.Page != page {
			return nil, unsupported("response page %d does not match requested page %d", env.Pagination.Page, page)
		}
		for _, raw := range env.Data {
			m, err := mapItem(raw)
			if err != nil {
				return nil, err
			}
			mergeModel(merged, m)
		}
		if !env.Pagination.HasMore {
			break
		}
		page++
	}

	models := make([]AAModel, 0, len(merged))
	for _, m := range merged {
		models = append(models, AAModel{
			Slug:                  m.Slug,
			IntelligenceIndex:     m.IntelligenceIndex,
			CodingIndex:           m.CodingIndex,
			AgenticIndex:          m.AgenticIndex,
			MedianResponseSeconds: m.MedianResponseSeconds,
			CostPerTaskUSD:        m.CostPerTaskUSD,
			Benchmarks:            m.Benchmarks,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].Slug < models[j].Slug })
	return models, nil
}

// mapItem converts one envelope data element. Evaluation values are
// fractions: x 100 then Round(2); negative values are a hard error.
func mapItem(raw json.RawMessage) (*mergedModel, error) {
	var item aaItem
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, wrapUnmarshalError(err)
	}
	if item.Slug == "" {
		return nil, unsupported("model record missing slug")
	}
	m := &mergedModel{Slug: item.Slug, Benchmarks: make(map[string]sdecimal.Decimal)}

	for _, f := range []struct {
		field string
		dst   **sdecimal.Decimal
	}{
		{"artificial_analysis_intelligence_index", &m.IntelligenceIndex},
		{"artificial_analysis_coding_index", &m.CodingIndex},
		{"artificial_analysis_agentic_index", &m.AgenticIndex},
	} {
		d, err := evalValue(item.Evaluations, f.field)
		if err != nil {
			return nil, err
		}
		if d == nil {
			continue
		}
		if d.Sign() < 0 {
			return nil, unsupported("negative value for %s", f.field)
		}
		pct := toPercent(*d)
		*f.dst = &pct
	}

	if p := item.Performance.MedianResponseSeconds; p != nil {
		if p.Sign() < 0 {
			return nil, unsupported("negative median response time")
		}
		m.MedianResponseSeconds = p
	}
	if c := item.Cost.CostPerTask.TotalCost; c != nil {
		if c.Sign() < 0 {
			return nil, unsupported("negative cost per task")
		}
		m.CostPerTaskUSD = c
	}

	for _, f := range AABenchmarkFields {
		d, err := evalValue(item.Evaluations, f.Field)
		if err != nil {
			return nil, err
		}
		if d == nil {
			continue
		}
		if d.Sign() < 0 {
			return nil, unsupported("negative value for %s", f.Field)
		}
		pct := toPercent(*d)
		if cur, ok := m.Benchmarks[f.Column]; !ok || pct.Cmp(cur) > 0 {
			m.Benchmarks[f.Column] = pct
		}
	}
	return m, nil
}

// evalValue extracts one evaluations.* value, handling absent keys and nulls
// as absent. shopspring's UnmarshalJSON accepts bare numbers and quoted
// numeric strings; anything else is unsupported_response.
func evalValue(evals map[string]json.RawMessage, field string) (*sdecimal.Decimal, error) {
	raw, ok := evals[field]
	if !ok {
		return nil, nil
	}
	if string(raw) == "null" {
		return nil, nil
	}
	var d sdecimal.Decimal
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, unsupported("evaluation %s is not numeric", field)
	}
	return &d, nil
}

func toPercent(d sdecimal.Decimal) sdecimal.Decimal {
	return wdecimal.RoundHalfUp(d.Mul(sdecimal.NewFromInt(100)), 2)
}

var (
	tauSuffixRE  = regexp.MustCompile(`-tau\d+$`)
	dateSuffixRE = regexp.MustCompile(`-(\d{8}|\d{4}-\d{2}-\d{2})$`)
)

// rootSlug strips a trailing -tau<digits> variant suffix, then a trailing
// -YYYYMMDD / -YYYY-MM-DD date suffix, then a trailing -latest
// (case-insensitive), at most once each (annex-b §2.4).
func rootSlug(slug string) string {
	for _, re := range []*regexp.Regexp{tauSuffixRE, dateSuffixRE} {
		if loc := re.FindStringIndex(slug); loc != nil {
			slug = slug[:loc[0]]
			break
		}
	}
	if strings.HasSuffix(strings.ToLower(slug), "-latest") {
		slug = slug[:len(slug)-len("-latest")]
	}
	return slug
}

// mergeModel deduplicates tau-variant records under their root slug,
// keeping the highest per converted value.
func mergeModel(merged map[string]*mergedModel, m *mergedModel) {
	key := rootSlug(m.Slug)
	if key != m.Slug {
		m.Slug = key
	}
	if existing, ok := merged[key]; ok {
		existing.IntelligenceIndex = maxDecimal(existing.IntelligenceIndex, m.IntelligenceIndex)
		existing.CodingIndex = maxDecimal(existing.CodingIndex, m.CodingIndex)
		existing.AgenticIndex = maxDecimal(existing.AgenticIndex, m.AgenticIndex)
		existing.MedianResponseSeconds = maxDecimal(existing.MedianResponseSeconds, m.MedianResponseSeconds)
		existing.CostPerTaskUSD = maxDecimal(existing.CostPerTaskUSD, m.CostPerTaskUSD)
		for col, v := range m.Benchmarks {
			if cur, ok := existing.Benchmarks[col]; !ok || v.Cmp(cur) > 0 {
				existing.Benchmarks[col] = v
			}
		}
		return
	}
	merged[key] = m
}

func maxDecimal(a, b *sdecimal.Decimal) *sdecimal.Decimal {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if b.Cmp(*a) > 0 {
		return b
	}
	return a
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

func unsupported(format string, args ...any) *fetch.Error {
	return &fetch.Error{Code: "unsupported_response", Err: fmt.Errorf(format, args...)}
}
