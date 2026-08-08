package aa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	sdecimal "github.com/shopspring/decimal"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch"
	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	wdecimal "github.com/WD-Mitchell/which-model/internal/decimal"
	"github.com/WD-Mitchell/which-model/internal/httpkit"
)

// PageMetrics is the extracted currentModel data from a model's public page.
type PageMetrics struct {
	Slug                           string
	TimePerIntelligenceTaskSeconds *sdecimal.Decimal // intelligenceIndexTimePerTask
	FallbackCostUSD                *sdecimal.Decimal // intelligenceIndexCostPerTask.cost.total (only when requireFallbackCost)
}

// ModelPageURL is the page URL for a slug.
func ModelPageURL(slug string) string {
	return "https://artificialanalysis.ai/models/" + slug
}

// markerData is the consumed subset of one currentModel marker.
type markerData struct {
	slug string
	time *sdecimal.Decimal
	cost *sdecimal.Decimal
}

// FetchAAPageFrom fetches the model's public page at url and extracts the
// currentModel marker matching slug (case-insensitively, variant suffixes
// stripped). Zero matching markers -> (nil, nil) with no error. Strict
// invariants: ambiguous or partial marker sets are errors, never guesses.
func FetchAAPageFrom(client *httpkit.Client, slug string, requireFallbackCost bool, url string) (*PageMetrics, error) {
	client.SetAllowList([]string{url})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, &fetch.Error{Code: "network", Err: err}
	}
	body, err := client.Do(context.Background(), req)
	if err != nil {
		return nil, wrapHTTPError(err)
	}

	markers, err := findCurrentModelMarkers(body, requireFallbackCost)
	if err != nil {
		return nil, unsupported("malformed currentModel marker: %v", err)
	}

	var matches []markerData
	for _, m := range markers {
		if slugMatches(m.slug, slug) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	for i := 1; i < len(matches); i++ {
		if matches[i].slug != matches[0].slug {
			return nil, unsupported("ambiguous currentModel markers for slug %s", slug)
		}
	}

	var times []sdecimal.Decimal
	var costs []sdecimal.Decimal
	for _, m := range matches {
		if m.time != nil {
			times = append(times, *m.time)
		}
		if requireFallbackCost && m.cost != nil {
			costs = append(costs, *m.cost)
		}
	}
	if len(times) == 0 {
		return nil, unsupported("matching currentModel marker carries no time for slug %s", slug)
	}
	if requireFallbackCost && len(times) != len(costs) {
		return nil, unsupported("currentModel marker counts disagree for slug %s", slug)
	}
	if !allEqual(times) {
		return nil, unsupported("conflicting currentModel time values for slug %s", slug)
	}
	if requireFallbackCost && !allEqual(costs) {
		return nil, unsupported("conflicting currentModel cost values for slug %s", slug)
	}

	pm := &PageMetrics{Slug: slug}
	t := times[0]
	pm.TimePerIntelligenceTaskSeconds = &t
	if requireFallbackCost {
		c := costs[0]
		pm.FallbackCostUSD = &c
	}
	return pm, nil
}

// FetchAAPage is the F23-facing wrapper over ModelPageURL(slug); F23 calls
// with requireFallbackCost=false (best effort).
func FetchAAPage(client *httpkit.Client, slug string, requireFallbackCost bool) (*PageMetrics, error) {
	return FetchAAPageFrom(client, slug, requireFallbackCost, ModelPageURL(slug))
}

// slugMatches reports whether a marker slug matches the requested slug:
// variant suffixes stripped, compared case-insensitively (T5: both
// "claude-opus-5" and "Claude-Opus-5-tau1" match "Claude-Opus-5").
// Slugs are canonicalized through the F07 collector-side cleaner first
// (F07 SPEC §1: "F08 collectors clean at ingestion"); AA slugs carry no
// bracket annotation groups, so for real payloads this is
// identity-neutral, but it keeps the F07 boundary pin (T7) on the path
// where model identity is decided.
func slugMatches(markerSlug, requested string) bool {
	return strings.EqualFold(
		rootSlug(identity.CleanModelName(markerSlug)),
		rootSlug(identity.CleanModelName(requested)),
	)
}

func allEqual(vals []sdecimal.Decimal) bool {
	for i := 1; i < len(vals); i++ {
		if vals[i].Cmp(vals[0]) != 0 {
			return false
		}
	}
	return true
}

// findCurrentModelMarkers scans raw HTML/JS for "currentModel": { ... }
// markers and strict-parses each balanced object. An unbalanced or
// duplicate-key object is a hard error.
func findCurrentModelMarkers(body []byte, requireFallbackCost bool) ([]markerData, error) {
	s := string(body)
	var markers []markerData
	for i := 0; i < len(s); {
		j := strings.Index(s[i:], `"currentModel"`)
		if j < 0 {
			break
		}
		k := i + j + len(`"currentModel"`)
		k = skipSpace(s, k)
		if k >= len(s) || s[k] != ':' {
			i = k
			continue
		}
		k = skipSpace(s, k+1)
		if k >= len(s) || s[k] != '{' {
			// Not an object marker ("currentModel": null etc.) — skip.
			i = k + 1
			continue
		}
		end := balancedObjectEnd(s, k)
		if end < 0 {
			return nil, errors.New("unbalanced object")
		}
		obj, err := parseStrictObject([]byte(s[k : end+1]))
		if err != nil {
			return nil, err
		}
		m, err := markerFrom(obj, requireFallbackCost)
		if err != nil {
			return nil, err
		}
		markers = append(markers, m)
		i = end + 1
	}
	return markers, nil
}

func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}

// balancedObjectEnd returns the index of the '}' closing the object that
// starts at s[start] ('{'), understanding quoted strings and escapes.
// Returns -1 when unbalanced.
func balancedObjectEnd(s string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// parseStrictObject decodes a JSON object, rejecting duplicate keys at any
// depth (annex §2.5) and decoding numbers as json.Number.
func parseStrictObject(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeStrictValue(dec, nil)
	if err != nil {
		return nil, err
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("marker is not an object")
	}
	return obj, nil
}

// decodeStrictValue walks one JSON value, rejecting duplicate object keys.
// tok == nil means "read the next token" (top level); otherwise tok is the
// already-read opening delimiter.
func decodeStrictValue(dec *json.Decoder, tok json.Token) (any, error) {
	if tok == nil {
		var err error
		tok, err = dec.Token()
		if err != nil {
			return nil, err
		}
	}
	switch d := tok.(type) {
	case json.Delim:
		switch d {
		case '{':
			obj := make(map[string]any)
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, errors.New("object key is not a string")
				}
				if _, dup := obj[key]; dup {
					return nil, fmt.Errorf("duplicate key %q", key)
				}
				val, err := decodeStrictValue(dec, nil)
				if err != nil {
					return nil, err
				}
				obj[key] = val
			}
			if _, err := dec.Token(); err != nil { // consume '}'
				return nil, err
			}
			return obj, nil
		case '[':
			var arr []any
			for dec.More() {
				val, err := decodeStrictValue(dec, nil)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := dec.Token(); err != nil { // consume ']'
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", d)
	default:
		return tok, nil
	}
}

// markerFrom extracts the consumed fields of one currentModel object. Cost
// is probed only when requireFallbackCost (the caller must opt in). Non-
// numeric (non-null) field values and negative numbers are hard errors.
func markerFrom(obj map[string]any, requireFallbackCost bool) (markerData, error) {
	var m markerData
	if slug, ok := obj["slug"].(string); ok {
		m.slug = slug
	}
	if raw, ok := obj["intelligenceIndexTimePerTask"]; ok && raw != nil {
		d, err := markerNumber(raw)
		if err != nil {
			return m, err
		}
		m.time = d
	}
	if !requireFallbackCost {
		return m, nil
	}
	if raw, ok := obj["intelligenceIndexCostPerTask"]; ok && raw != nil {
		costObj, ok := raw.(map[string]any)
		if !ok {
			return m, unsupported("intelligenceIndexCostPerTask is not an object")
		}
		cost, ok := costObj["cost"]
		if !ok || cost == nil {
			return m, nil // no cost data carried
		}
		costFields, ok := cost.(map[string]any)
		if !ok {
			return m, unsupported("intelligenceIndexCostPerTask.cost is not an object")
		}
		total, ok := costFields["total"]
		if !ok || total == nil {
			return m, nil // no cost data carried
		}
		d, err := markerNumber(total)
		if err != nil {
			return m, err
		}
		m.cost = d
	}
	return m, nil
}

func markerNumber(v any) (*sdecimal.Decimal, error) {
	n, ok := v.(json.Number)
	if !ok {
		return nil, unsupported("non-numeric marker value")
	}
	d, err := wdecimal.Parse(n.String())
	if err != nil {
		return nil, unsupported("non-numeric marker value")
	}
	if d.Sign() < 0 {
		return nil, unsupported("negative marker value")
	}
	return &d, nil
}
