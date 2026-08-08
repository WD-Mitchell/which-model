package modelsdev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"

	sdecimal "github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// BenchmarksURL is the models.dev benchmarks endpoint, distinct from the
// providers endpoint. It is the exact URL pinned by the F08 tests and the
// allow-list entry for every request.
const BenchmarksURL = "https://models.dev/models.json"

// BenchmarkEvidence is one selected benchmark's score for a model, scoped by
// the record's reasoning-effort variant ("" when the record has no variant
// or an unrecognized one).
type BenchmarkEvidence struct {
	Name   string
	Score  sdecimal.Decimal
	Effort string
}

// BenchmarkRecord is one models.dev benchmark record, keyed by the model's
// canonical id. Records sharing a canonical id are merged: for each
// (benchmark name, effort) pair only the maximum score is kept.
type BenchmarkRecord struct {
	CanonicalID string
	Name        string
	Benchmarks  []BenchmarkEvidence
}

// benchmarkRecordKeys are the record-level fields excluded from the benchmark
// scan (annex-b §2.4).
var benchmarkRecordKeys = map[string]bool{"id": true, "name": true, "variant": true}

type evidenceKey struct {
	name   string
	effort string
}

type benchmarkAccum struct {
	rec  *BenchmarkRecord
	best map[evidenceKey]sdecimal.Decimal
}

// FetchModelsDevBenchmarksFrom fetches the models.dev benchmarks endpoint at
// url and keeps only the benchmark fields named in selectedNames.
func FetchModelsDevBenchmarksFrom(client *httpkit.Client, url string, selectedNames []string) ([]BenchmarkRecord, error) {
	client.SetAllowList([]string{url})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, &fetch.Error{Code: "network", Err: err}
	}
	req.Header.Set("Accept", "application/json")
	body, err := client.Do(context.Background(), req)
	if err != nil {
		return nil, benchHTTPError(err)
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var records []map[string]any
	if err := dec.Decode(&records); err != nil {
		return nil, benchUnmarshalError(err)
	}

	selected := make(map[string]bool, len(selectedNames))
	for _, n := range selectedNames {
		selected[n] = true
	}

	accs := make(map[string]*benchmarkAccum, len(records))
	for _, rec := range records {
		if rec == nil {
			return nil, benchUnsupported("null model record")
		}
		id, ok := rec["id"].(string)
		if !ok || id == "" {
			return nil, benchUnsupported("model record missing id")
		}
		name, _ := rec["name"].(string)

		effort := ""
		if v, ok := rec["variant"].(string); ok {
			if lvl, ok := identity.ParseEffort(v); ok {
				effort = lvl
			}
		}

		acc, ok := accs[id]
		if !ok {
			acc = &benchmarkAccum{
				rec:  &BenchmarkRecord{CanonicalID: id, Name: name},
				best: make(map[evidenceKey]sdecimal.Decimal),
			}
			accs[id] = acc
		}

		// Deterministic key order keeps behavior independent of map iteration.
		names := make([]string, 0, len(rec))
		for k := range rec {
			if !benchmarkRecordKeys[k] && selected[k] {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		for _, k := range names {
			score, err := benchmarkScore(rec[k])
			if err != nil {
				return nil, err
			}
			key := evidenceKey{name: k, effort: effort}
			if cur, ok := acc.best[key]; !ok || score.Cmp(cur) > 0 {
				acc.best[key] = score
			}
		}
	}

	ids := make([]string, 0, len(accs))
	for id := range accs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]BenchmarkRecord, 0, len(accs))
	for _, id := range ids {
		acc := accs[id]
		keys := make([]evidenceKey, 0, len(acc.best))
		for k := range acc.best {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].name != keys[j].name {
				return keys[i].name < keys[j].name
			}
			return keys[i].effort < keys[j].effort
		})
		for _, k := range keys {
			acc.rec.Benchmarks = append(acc.rec.Benchmarks, BenchmarkEvidence{Name: k.name, Score: acc.best[k], Effort: k.effort})
		}
		out = append(out, *acc.rec)
	}
	return out, nil
}

// FetchModelsDevBenchmarks is the production wrapper using the pinned
// BenchmarksURL and a default httpkit client.
func FetchModelsDevBenchmarks(selectedNames []string) ([]BenchmarkRecord, error) {
	return FetchModelsDevBenchmarksFrom(httpkit.NewClient(), BenchmarksURL, selectedNames)
}

// benchmarkScore normalizes one benchmark entry: a bare number, a numeric
// string, or an object {"score": <number|numeric string>, "version": ...}.
func benchmarkScore(v any) (sdecimal.Decimal, error) {
	switch t := v.(type) {
	case json.Number:
		return parseScore(t.String())
	case string:
		return parseScore(t)
	case map[string]any:
		score, ok := t["score"]
		if !ok {
			return sdecimal.Zero, benchUnsupported("benchmark entry missing score")
		}
		return benchmarkScore(score)
	default:
		return sdecimal.Zero, benchUnsupported("benchmark value is not numeric")
	}
}

func parseScore(s string) (sdecimal.Decimal, error) {
	d, err := wdecimal.Parse(s)
	if err != nil {
		return sdecimal.Zero, benchUnsupported("benchmark score %q is not numeric", s)
	}
	if d.Sign() < 0 {
		return sdecimal.Zero, benchUnsupported("benchmark score is negative")
	}
	return d, nil
}

// The helpers below are benchmark.go's own copies of the fetch error-mapping
// helpers; benchmark.go must not reference provider-file symbols (T3 file
// isolation), so they are duplicated here with file-local names.

// benchHTTPError maps an *httpkit.Error to its global Failure.Code.
func benchHTTPError(err error) *fetch.Error {
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

// benchUnmarshalError classifies a JSON decode failure: syntax problems are
// response_json; shape/type problems are unsupported_response.
func benchUnmarshalError(err error) *fetch.Error {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) {
		return &fetch.Error{Code: "unsupported_response", Err: err}
	}
	return &fetch.Error{Code: "response_json", Err: err}
}

func benchUnsupported(format string, args ...any) *fetch.Error {
	return &fetch.Error{Code: "unsupported_response", Err: fmt.Errorf(format, args...)}
}
