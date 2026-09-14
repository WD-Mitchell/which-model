package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

const releaseTestSite = "https://github.com/WD-Mitchell/which-model/releases"

type releaseTestTransport func(*http.Request) (*http.Response, error)

func (f releaseTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func releaseTestResponse(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

func TestReleaseUpdate(t *testing.T) {
	tests := []struct {
		name, current, releases, message, link string
		requests                               int
	}{
		{"numeric prerelease", "2.5.5", `[{"tag_name":"v2.5.6","prerelease":true},{"tag_name":"v2.5.5"}]`, "update available: v2.5.6 (you have 2.5.5)", releaseTestSite + "/tag/v2.5.6", 1},
		{"historical latest is older", "2.5.6", `[{"tag_name":"v2.5.5"}]`, "no newer release available (you have 2.5.6)", "", 1},
		{"same version ignores prefix and build metadata", "2.5.6+local", `[{"tag_name":"v2.5.6","prerelease":true}]`, "up to date (2.5.6+local)", "", 1},
		{"numeric ordering independent of list order", "2.9.0", `[{"tag_name":"v2.9.0"},{"tag_name":"v2.10.0","prerelease":true},{"tag_name":"v2.8.0"}]`, "update available: v2.10.0 (you have 2.9.0)", releaseTestSite + "/tag/v2.10.0", 1},
		{"suffix numeric ordering", "v2.6.0-rc.2", `[{"tag_name":"v2.6.0-rc.10","prerelease":true},{"tag_name":"v2.6.0-rc.3","prerelease":true}]`, "update available: v2.6.0-rc.10 (you have v2.6.0-rc.2)", releaseTestSite + "/tag/v2.6.0-rc.10", 1},
		{"final version exceeds its suffix", "2.6.0-rc.10", `[{"tag_name":"v2.6.0","prerelease":true}]`, "update available: v2.6.0 (you have 2.6.0-rc.10)", releaseTestSite + "/tag/v2.6.0", 1},
		{"ignore drafts and nonversion tags", "2.5.5", `[{"tag_name":"v9.0.0","draft":true},{"tag_name":"nightly"},{"tag_name":"v8"},{"tag_name":"v7.1"},{"tag_name":"v2.5.6"}]`, "update available: v2.5.6 (you have 2.5.5)", releaseTestSite + "/tag/v2.5.6", 1},
		{"development build", "dev", "", "development build; browse releases to check for updates", releaseTestSite, 0},
		{"empty version", "", "", "development build; browse releases to check for updates", releaseTestSite, 0},
		{"unknown version", "custom-build", "", "unrecognized build version; browse releases to check for updates", releaseTestSite, 0},
		{"incomplete version", "2.5", "", "unrecognized build version; browse releases to check for updates", releaseTestSite, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: releaseTestTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if want := "https://api.github.com/repos/WD-Mitchell/which-model/releases?per_page=100&page=1"; r.URL.String() != want {
					t.Errorf("release lookup = %s, want %s", r.URL, want)
				}
				return releaseTestResponse(tt.releases, http.StatusOK), nil
			})}
			message, link, err := checkReleaseUpdate(context.Background(), client, tt.current)
			if err != nil || message != tt.message || link != tt.link || calls != tt.requests {
				t.Fatalf("got (%q, %q, %v), %d requests; want (%q, %q, nil), %d requests", message, link, err, calls, tt.message, tt.link, tt.requests)
			}
		})
	}
}

func TestReleaseUpdatePagination(t *testing.T) {
	// Creation order need not match version order: the higher version is on
	// the next page, following a full page of older maintenance releases.
	old := make([]map[string]string, 100)
	for i := range old {
		old[i] = map[string]string{"tag_name": fmt.Sprintf("v1.0.%d", i)}
	}
	firstPage, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := &http.Client{Transport: releaseTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Query().Get("page") != fmt.Sprint(calls) {
			t.Errorf("unexpected page: %s", r.URL)
		}
		if calls == 1 {
			return releaseTestResponse(string(firstPage), http.StatusOK), nil
		}
		return releaseTestResponse(`[{"tag_name":"v2.5.6","prerelease":true}]`, http.StatusOK), nil
	})}
	message, link, err := checkReleaseUpdate(context.Background(), client, "2.5.5")
	if err != nil || message != "update available: v2.5.6 (you have 2.5.5)" || link != releaseTestSite+"/tag/v2.5.6" || calls != 2 {
		t.Fatalf("got (%q, %q, %v), %d requests", message, link, err, calls)
	}
}

func TestReleaseUpdateFailures(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
	}{
		{"rate limited", "{}", http.StatusForbidden},
		{"bad JSON", "broken", http.StatusOK},
		{"wrong shape", `{"tag_name":"v2.5.6"}`, http.StatusOK},
		{"null list", "null", http.StatusOK},
		{"empty list", "[]", http.StatusOK},
		{"no eligible release", `[{"tag_name":"nightly"},{"tag_name":"v9.0.0","draft":true}]`, http.StatusOK},
		{"oversized response", strings.Repeat(" ", 4*1024*1024+1), http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: releaseTestTransport(func(*http.Request) (*http.Response, error) {
				return releaseTestResponse(tt.body, tt.status), nil
			})}
			message, link, err := checkReleaseUpdate(context.Background(), client, "2.5.5")
			if err == nil || message != "" || link != "" {
				t.Fatalf("got (%q, %q, %v); incomplete evidence must not claim an update or up-to-date state", message, link, err)
			}
		})
	}
	t.Run("cancelled request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := &http.Client{Transport: releaseTestTransport(func(r *http.Request) (*http.Response, error) {
			return nil, r.Context().Err()
		})}
		message, link, err := checkReleaseUpdate(ctx, client, "2.5.5")
		if !errors.Is(err, context.Canceled) || message != "" || link != "" {
			t.Fatalf("got (%q, %q, %v)", message, link, err)
		}
	})
}

func TestReleaseUpdateIncompletePagination(t *testing.T) {
	fullPage := "[" + strings.Repeat(`{"tag_name":"v2.5.6"},`, 99) + `{"tag_name":"v2.5.6"}]`
	for _, failSecondPage := range []bool{true, false} {
		t.Run(fmt.Sprintf("network_failure_%v", failSecondPage), func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: releaseTestTransport(func(*http.Request) (*http.Response, error) {
				calls++
				if failSecondPage && calls > 1 {
					return nil, errors.New("network unavailable")
				}
				return releaseTestResponse(fullPage, http.StatusOK), nil
			})}
			message, link, err := checkReleaseUpdate(context.Background(), client, "2.5.6")
			if err == nil || message != "" || link != "" || calls > 10 {
				t.Fatalf("got (%q, %q, %v), %d requests; incomplete listings cannot claim up to date", message, link, err, calls)
			}
		})
	}
}
