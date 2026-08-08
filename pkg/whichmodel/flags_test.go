package whichmodel

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestFlagsDefaults(t *testing.T) {
	var g GlobalFlags
	cmd := &cobra.Command{Use: "x"}
	if err := g.Bind(cmd); err != nil {
		t.Fatal(err)
	}
	if g.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", g.Timeout)
	}
	if g.Normalizer != "minmax-linear" {
		t.Errorf("Normalizer = %q", g.Normalizer)
	}
	if g.Aggregator != "weighted-arithmetic-mean" {
		t.Errorf("Aggregator = %q", g.Aggregator)
	}
	for name, val := range map[string]bool{
		"json": g.JSON, "text": g.Text, "quiet": g.Quiet, "no-color": g.NoColor,
		"offline": g.Offline, "refresh-usage": g.RefreshUsage,
		"refresh-benchmarks": g.RefreshBenchmarks, "refresh-scores": g.RefreshScores,
		"refresh": g.Refresh, "no-usage": g.NoUsage, "show-identity": g.ShowIdentity,
		"schema": g.Schema, "version": g.Version,
	} {
		if val {
			t.Errorf("flag %s default = true, want false", name)
		}
	}
	if g.Verbose != 0 {
		t.Errorf("Verbose default = %d, want 0", g.Verbose)
	}
	if g.MaxAge != 0 {
		t.Errorf("MaxAge default = %v, want 0", g.MaxAge)
	}
	if g.ConfigPath != "" {
		t.Errorf("ConfigPath default = %q, want empty", g.ConfigPath)
	}
}

func TestFlagsNormalize(t *testing.T) {
	g := GlobalFlags{Refresh: true}
	if err := g.Normalize(); err != nil {
		t.Fatal(err)
	}
	if !g.RefreshUsage || !g.RefreshBenchmarks || !g.RefreshScores {
		t.Error("Normalize must set all three refresh flags")
	}

	g2 := GlobalFlags{RefreshUsage: true, Refresh: true}
	if err := g2.Normalize(); err != nil {
		t.Fatal(err)
	}
	if !g2.RefreshUsage || !g2.RefreshBenchmarks || !g2.RefreshScores {
		t.Error("Normalize must be idempotent on already-true fields")
	}
}

func TestFlagsValidate(t *testing.T) {
	tests := []struct {
		name    string
		g       GlobalFlags
		wantErr bool
	}{
		{"json+text", GlobalFlags{JSON: true, Text: true}, true},
		{"offline+refresh", GlobalFlags{Offline: true, Refresh: true}, true},
		{"offline+refresh-benchmarks", GlobalFlags{Offline: true, RefreshBenchmarks: true}, true},
		{"offline+refresh-scores allowed", GlobalFlags{Offline: true, RefreshScores: true}, false},
		{"clean", GlobalFlags{RefreshScores: true}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.g.Validate()
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected UsageError")
				}
				if _, ok := err.(*UsageError); !ok {
					t.Errorf("error type = %T, want *UsageError", err)
				}
			} else if err != nil {
				t.Errorf("err = %v, want nil", err)
			}
		})
	}
}

func TestFlagsBind(t *testing.T) {
	var g GlobalFlags
	cmd := &cobra.Command{Use: "x"}
	if err := g.Bind(cmd); err != nil {
		t.Fatal(err)
	}
	if got := cmd.PersistentFlags().Lookup("timeout").DefValue; got != "10s" {
		t.Errorf("timeout DefValue = %q, want 10s", got)
	}
	count := 0
	cmd.PersistentFlags().VisitAll(func(*pflag.Flag) { count++ })
	if count != 18 {
		t.Errorf("persistent flags = %d, want 18", count)
	}
}
