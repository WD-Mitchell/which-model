//go:build !nousage

package credential

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestParseExpiry(t *testing.T) {
	// 1700000000 s == 2023-11-14T22:13:20Z; the same value in ms is
	// disambiguated by the > 10_000_000_000 heuristic (SPEC §12).
	nov2023 := time.Date(2023, 11, 14, 22, 13, 20, 0, time.UTC)

	cases := []struct {
		name string
		in   any
		want time.Time
	}{
		{"float64 seconds", float64(1_700_000_000), nov2023},       // case 1
		{"float64 milliseconds", float64(1_700_000_000_000), nov2023}, // case 2
		{"int seconds", int(1_700_000_000), nov2023},
		{"int64 seconds", int64(1_700_000_000), nov2023},
		{"json.Number milliseconds", json.Number("1700000000000"), nov2023},
		{"rfc3339 string", "2026-01-02T15:04:05Z", time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)}, // case 3
		{"rfc3339nano string", "2026-01-02T15:04:05.123456789Z", time.Date(2026, 1, 2, 15, 4, 5, 123456789, time.UTC)},
		{"numeric string", "1700000000", nov2023}, // case 4
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseExpiry(tc.in)
			if err != nil {
				t.Fatalf("ParseExpiry(%v) error = %v, want %v", tc.in, err, tc.want)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("ParseExpiry(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseExpiryErrors(t *testing.T) {
	cases := []struct {
		name string
		in   any
	}{
		{"unparseable string", "soon"}, // case 5
		{"empty string", ""},
		{"bool", true}, // case 6
		{"nil", nil},
		{"object", map[string]any{"x": 1}},
		{"slice", []any{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseExpiry(tc.in); err == nil {
				t.Fatalf("ParseExpiry(%v) = nil error, want error", tc.in)
			}
		})
	}
}

func TestCheckExpired(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	t.Run("past expiry", func(t *testing.T) { // case 7
		err := CheckExpired(now.Add(-1*time.Hour), now)
		if err == nil {
			t.Fatal("CheckExpired(past, now) = nil, want expired_credential")
		}
		f, ok := usage.AsFailure(err)
		if !ok || f.Code != "expired_credential" {
			t.Fatalf("CheckExpired(past, now) = %v, want expired_credential FailureError", err)
		}
	})

	t.Run("future expiry", func(t *testing.T) { // case 8
		if err := CheckExpired(now.Add(1*time.Hour), now); err != nil {
			t.Fatalf("CheckExpired(future, now) = %v, want nil", err)
		}
	})

	t.Run("exactly now is not expired", func(t *testing.T) {
		if err := CheckExpired(now, now); err != nil {
			t.Fatalf("CheckExpired(now, now) = %v, want nil (expiry at now is still usable)", err)
		}
	})

	t.Run("expired is a FailureError, not ErrNotFound", func(t *testing.T) {
		if errors.Is(CheckExpired(now.Add(-time.Hour), now), ErrNotFound) {
			t.Fatal("expired_credential must not be ErrNotFound")
		}
	})
}
