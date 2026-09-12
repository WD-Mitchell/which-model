package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// GitHub Latest excludes prereleases, including this project's numeric
	// releases. Discover published versions from the complete release list.
	releasesAPIURL      = "https://api.github.com/repos/WD-Mitchell/which-model/releases"
	releasesPageURL     = "https://github.com/WD-Mitchell/which-model/releases"
	updateCheckTimeout  = 10 * time.Second
	releasePageSize     = 100
	releaseMaxPages     = 10
	releaseMaxPageBytes = 4 * 1024 * 1024
)

type publishedRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
}

// checkReleaseUpdate supplies the tray's notice and optional browser link.
// Maturity flags do not affect version ordering; this app is pre-release.
// It never installs a binary or changes the user's selected version.
func checkReleaseUpdate(ctx context.Context, client *http.Client, current string) (message, link string, err error) {
	if current == "" || current == "dev" {
		return "development build; browse releases to check for updates", releasesPageURL, nil
	}
	version := releaseVersion(current)
	if version == "" {
		return "unrecognized build version; browse releases to check for updates", releasesPageURL, nil
	}
	latest, err := latestPublishedRelease(ctx, client)
	if err != nil {
		return "", "", err
	}
	switch semver.Compare(releaseVersion(latest), version) {
	case 1:
		return fmt.Sprintf("update available: %s (you have %s)", latest, current), releasesPageURL + "/tag/" + url.PathEscape(latest), nil
	case 0:
		return fmt.Sprintf("up to date (%s)", current), "", nil
	default:
		return fmt.Sprintf("no newer release available (you have %s)", current), "", nil
	}
}

// releaseVersion accepts full semantic versions with an optional v prefix.
// x/mod also accepts v1 and v1.2; those do not identify our release builds.
func releaseVersion(tag string) string {
	version := "v" + strings.TrimPrefix(tag, "v")
	core, _, _ := strings.Cut(version, "-")
	core, _, _ = strings.Cut(core, "+")
	if strings.Count(core, ".") != 2 || !semver.IsValid(version) {
		return ""
	}
	return version
}

func latestPublishedRelease(ctx context.Context, client *http.Client) (string, error) {
	latest := ""
	for page := 1; page <= releaseMaxPages; page++ {
		releases, err := fetchReleasePage(ctx, client, page)
		if err != nil {
			return "", err
		}
		for _, release := range releases {
			version := releaseVersion(release.TagName)
			if !release.Draft && version != "" && (latest == "" || semver.Compare(version, releaseVersion(latest)) > 0) {
				latest = release.TagName
			}
		}
		if len(releases) < releasePageSize {
			if latest == "" {
				return "", fmt.Errorf("no published semantic-version release")
			}
			return latest, nil
		}
	}
	return "", fmt.Errorf("release listing exceeds %d pages", releaseMaxPages)
}

func fetchReleasePage(ctx context.Context, client *http.Client, page int) ([]publishedRelease, error) {
	endpoint := fmt.Sprintf("%s?per_page=%d&page=%d", releasesAPIURL, releasePageSize, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, releaseMaxPageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > releaseMaxPageBytes {
		return nil, fmt.Errorf("release response exceeds %d bytes", releaseMaxPageBytes)
	}
	var releases []publishedRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, err
	}
	if releases == nil || len(releases) > releasePageSize {
		return nil, fmt.Errorf("invalid release list")
	}
	return releases, nil
}
