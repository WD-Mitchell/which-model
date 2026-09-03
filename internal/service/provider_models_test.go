package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

func TestParseCursorModelList(t *testing.T) {
	output := `Available models

auto - Auto (current, default)
gpt-5.6-sol-high-fast - GPT-5.6 Sol 1M High Fast
claude-fable-5-max - Claude Fable 5 1M Max (NO ZDR)
claude-fable-5-thinking-max - Claude Fable 5 1M Max Thinking (NO ZDR)
cursor-grok-4.6-high - Cursor Grok 4.6
claude-opus-5-medium - Claude Opus 5 Medium
claude-opus-5-medium-fast - Claude Opus 5 Medium Fast
claude-opus-5-thinking-high - Claude Opus 5 High Thinking
claude-opus-5-thinking-high-fast - Claude Opus 5 High Thinking Fast
claude-opus-5-low-fast - Claude Opus 5 Low Fast

Tip: use --model <id> to switch.
`
	got, err := parseCursorModelList(output)
	if err != nil {
		t.Fatalf("parseCursorModelList() error = %v", err)
	}
	want := []routing.ModelEntry{
		{ModelID: "gpt-5.6-sol", Name: "GPT-5.6 Sol", Reasoning: []string{"high"}},
		{ModelID: "claude-fable-5", Name: "Claude Fable 5"},
		{ModelID: "cursor-grok-4.6", Name: "Grok 4.6", Reasoning: []string{"high"}},
		{ModelID: "claude-opus-5", Name: "Claude Opus 5", Reasoning: []string{"low", "medium", "high"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCursorModelList() = %#v, want %#v", got, want)
	}
}
func TestParseCursorModelList_ComprehensiveVariants(t *testing.T) {
	output := `Available models

auto - Auto (current, default)
gpt-5.3-codex-low - Codex 5.3 Low
gpt-5.3-codex-low-fast - Codex 5.3 Low Fast
gpt-5.3-codex - Codex 5.3
gpt-5.3-codex-fast - Codex 5.3 Fast
gpt-5.3-codex-high - Codex 5.3 High
gpt-5.3-codex-high-fast - Codex 5.3 High Fast
gpt-5.3-codex-xhigh - Codex 5.3 Extra High
gpt-5.3-codex-xhigh-fast - Codex 5.3 Extra High Fast
cursor-grok-4.6-low - Cursor Grok 4.6 Low
cursor-grok-4.6-low-fast - Cursor Grok 4.6 Low Fast
cursor-grok-4.6-medium - Cursor Grok 4.6 Medium
cursor-grok-4.6-medium-fast - Cursor Grok 4.6 Medium Fast
cursor-grok-4.6-high - Cursor Grok 4.6
cursor-grok-4.6-high-fast - Cursor Grok 4.6 Fast
cursor-grok-4.6-xhigh - Cursor Grok 4.6 Extra High
cursor-grok-4.6-xhigh-fast - Cursor Grok 4.6 Extra High Fast
composer-2.5 - Composer 2.5
composer-2.5-fast - Composer 2.5 Fast
claude-opus-4-8-low - Claude Opus 4.8 1M Low
claude-opus-4-8-low-fast - Claude Opus 4.8 1M Low Fast
claude-opus-4-8-medium - Claude Opus 4.8 1M Medium
claude-opus-4-8-medium-fast - Claude Opus 4.8 1M Medium Fast
claude-opus-4-8-high - Claude Opus 4.8 1M
claude-opus-4-8-high-fast - Claude Opus 4.8 1M Fast
claude-opus-4-8-xhigh - Claude Opus 4.8 1M Extra High
claude-opus-4-8-xhigh-fast - Claude Opus 4.8 1M Extra High Fast
claude-opus-4-8-max - Claude Opus 4.8 1M Max
claude-opus-4-8-max-fast - Claude Opus 4.8 1M Max Fast
claude-opus-4-8-thinking-low - Claude Opus 4.8 1M Low Thinking
claude-opus-4-8-thinking-low-fast - Claude Opus 4.8 1M Low Thinking Fast
claude-opus-4-8-thinking-medium - Claude Opus 4.8 1M Medium Thinking
claude-opus-4-8-thinking-medium-fast - Claude Opus 4.8 1M Medium Thinking Fast
claude-opus-4-8-thinking-high - Claude Opus 4.8 1M Thinking
claude-opus-4-8-thinking-high-fast - Claude Opus 4.8 1M Thinking Fast
claude-opus-4-8-thinking-xhigh - Claude Opus 4.8 1M Extra High Thinking
claude-opus-4-8-thinking-xhigh-fast - Claude Opus 4.8 1M Extra High Thinking Fast
claude-opus-4-8-thinking-max - Claude Opus 4.8 1M Max Thinking
claude-opus-4-8-thinking-max-fast - Claude Opus 4.8 1M Max Thinking Fast
claude-4.6-sonnet-medium - Claude Sonnet 4.6 1M
claude-4.6-sonnet-medium-thinking - Claude Sonnet 4.6 1M Thinking
claude-4.6-opus-high - Claude Opus 4.6 1M
claude-4.6-opus-max - Claude Opus 4.6 1M Max
claude-4.6-opus-high-thinking - Claude Opus 4.6 1M Thinking
claude-4.6-opus-max-thinking - Claude Opus 4.6 1M Max Thinking
claude-4.5-sonnet - Claude Sonnet 4.5
claude-4.5-sonnet-thinking - Claude Sonnet 4.5 Thinking
kimi-k3-low - Kimi K3 Low
kimi-k3-high - Kimi K3 High
kimi-k3-max - Kimi K3

Tip: use --model <id> to switch.
`
	got, err := parseCursorModelList(output)
	if err != nil {
		t.Fatalf("parseCursorModelList() error = %v", err)
	}
	want := []routing.ModelEntry{
		{ModelID: "gpt-5.3-codex", Name: "Codex 5.3", Reasoning: []string{"low", "high", "xhigh"}},
		{ModelID: "cursor-grok-4.6", Name: "Grok 4.6", Reasoning: []string{"low", "medium", "high", "xhigh"}},
		{ModelID: "composer-2.5", Name: "Composer 2.5"},
		{ModelID: "claude-opus-4-8", Name: "Claude Opus 4.8", Reasoning: []string{"low", "medium", "high", "xhigh"}},
		{ModelID: "claude-4.6-sonnet", Name: "Claude Sonnet 4.6", Reasoning: []string{"medium"}},
		{ModelID: "claude-4.6-opus", Name: "Claude Opus 4.6", Reasoning: []string{"high"}},
		{ModelID: "claude-4.5-sonnet", Name: "Claude Sonnet 4.5"},
		{ModelID: "kimi-k3", Name: "Kimi K3", Reasoning: []string{"low", "high"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCursorModelList() = %#v, want %#v", got, want)
	}
}


func TestParseAntigravityModelList(t *testing.T) {
	output := "Fetching available models...\n" +
		"gemini-3.6-flash-high\tGemini 3.6 Flash (High)\n" +
		"claude-sonnet-4-6\tClaude Sonnet 4.6 (Thinking)\n" +
		"gpt-oss-120b-medium\tGPT-OSS 120B (Medium)\n"
	got, err := parseAntigravityModelList(output)
	if err != nil {
		t.Fatalf("parseAntigravityModelList() error = %v", err)
	}
	want := []routing.ModelEntry{
		{ModelID: "gemini-3.6-flash-high", Name: "Gemini 3.6 Flash", Reasoning: []string{"high"}},
		{ModelID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6"},
		{ModelID: "gpt-oss-120b-medium", Name: "GPT-OSS 120B", Reasoning: []string{"medium"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseAntigravityModelList() = %#v, want %#v", got, want)
	}
}

func TestProviderModelListRejectsMalformedOutput(t *testing.T) {
	if got, err := parseCursorModelList("Available models\nnot a model record\n"); err == nil || got != nil {
		t.Fatalf("parseCursorModelList(malformed) = %#v, %v, want nil and error", got, err)
	}
	if got, err := parseAntigravityModelList("Fetching available models...\nnot-a-record\n"); err == nil || got != nil {
		t.Fatalf("parseAntigravityModelList(malformed) = %#v, %v, want nil and error", got, err)
	}
	if got, err := parseCursorModelList("Available models\ngpt-5.6-sol - GPT-5.6 Sol\ngpt-5.6-sol - GPT-5.6 Sol\n"); err == nil || got != nil {
		t.Fatalf("parseCursorModelList(duplicate raw row) = %#v, %v, want nil and error", got, err)
	}
	if got, err := parseAntigravityModelList("Fetching available models...\ngpt-oss-120b-medium\tGPT-OSS 120B\ngpt-oss-120b-medium\tGPT-OSS 120B\n"); err == nil || got != nil {
		t.Fatalf("parseAntigravityModelList(duplicate raw row) = %#v, %v, want nil and error", got, err)
	}
}

func TestProviderModelOutputCapsBytes(t *testing.T) {
	var output providerModelOutput
	first := make([]byte, maxProviderModelOutputBytes-1)
	if n, err := output.Write(first); err != nil || n != len(first) {
		t.Fatalf("Write(first) = %d, %v; want %d, nil", n, err, len(first))
	}
	overflow := []byte("overflow")
	if n, err := output.Write(overflow); err != nil || n != len(overflow) {
		t.Fatalf("Write(overflow) = %d, %v; want %d, nil", n, err, len(overflow))
	}
	if !output.tooLarge {
		t.Fatal("tooLarge = false, want true")
	}
	if got := output.Len(); got != maxProviderModelOutputBytes {
		t.Fatalf("Len() = %d, want %d", got, maxProviderModelOutputBytes)
	}
}

func TestDiscoverLiveProviderModelsUsesProviderCommandsAndFallback(t *testing.T) {
	previous := runProviderModelCommand
	t.Cleanup(func() { runProviderModelCommand = previous })

	type call struct {
		binary string
		args   []string
	}
	var calls []call
	runProviderModelCommand = func(_ context.Context, binary string, args ...string) ([]byte, error) {
		calls = append(calls, call{binary: binary, args: append([]string(nil), args...)})
		switch binary {
		case "cursor-agent":
			return []byte("Available models\ngpt-5.6-sol-high - GPT-5.6 Sol 1M High\n"), nil
		case "agy":
			return nil, errors.New("agy unavailable")
		case "antigravity":
			return []byte("gemini-3.6-flash-low\tGemini 3.6 Flash (Low)\n"), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}

	cursor := discoverLiveProviderModelsDefault(context.Background(), "cursor")
	if want := []routing.ModelEntry{{ModelID: "gpt-5.6-sol", Name: "GPT-5.6 Sol", Reasoning: []string{"high"}}}; !reflect.DeepEqual(cursor, want) {
		t.Fatalf("cursor models = %#v, want %#v", cursor, want)
	}
	antigravity := discoverLiveProviderModelsDefault(context.Background(), "antigravity")
	if want := []routing.ModelEntry{{ModelID: "gemini-3.6-flash-low", Name: "Gemini 3.6 Flash", Reasoning: []string{"low"}}}; !reflect.DeepEqual(antigravity, want) {
		t.Fatalf("antigravity models = %#v, want %#v", antigravity, want)
	}
	wantCalls := []call{
		{binary: "cursor-agent", args: []string{"--list-models"}},
		{binary: "agy", args: []string{"models"}},
		{binary: "antigravity", args: []string{"models"}},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("commands = %#v, want %#v", calls, wantCalls)
	}
}

func TestDiscoverLiveProviderModelsFailsClosed(t *testing.T) {
	previous := runProviderModelCommand
	t.Cleanup(func() { runProviderModelCommand = previous })

	cases := []struct {
		name   string
		output []byte
		err    error
	}{
		{name: "command failure", err: errors.New("command failed")},
		{name: "empty output"},
		{name: "malformed output", output: []byte("Available models\nnot a model record\n")},
		{name: "duplicate model id", output: []byte("Available models\nm1 - Model One\nm1 - Model One\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runProviderModelCommand = func(context.Context, string, ...string) ([]byte, error) {
				return tc.output, tc.err
			}
			if got := discoverLiveProviderModelsDefault(context.Background(), "cursor"); got != nil {
				t.Fatalf("discoverLiveProviderModelsDefault(cursor) = %#v, want nil", got)
			}
			if got := discoverLiveProviderModelsDefault(context.Background(), "antigravity"); got != nil {
				t.Fatalf("discoverLiveProviderModelsDefault(antigravity) = %#v, want nil", got)
			}
		})
	}
	if got := discoverLiveProviderModelsDefault(context.Background(), "claude"); got != nil {
		t.Fatalf("discoverLiveProviderModelsDefault(claude) = %#v, want nil", got)
	}
}

func TestRefreshRoutesDiscoversCursorAndAntigravityWithoutOpencodeAmbiguity(t *testing.T) {
	const scores = "model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score\n" +
		"GPT-5.6 Sol,medium,100,100,100,100,100,100\n" +
		"Gemini 3.6 Flash,high,100,100,100,100,100,100\n" +
		"Kimi K3,low,100,100,100,100,100,100\n" +
		"Kimi K3,max,100,100,100,100,100,100\n"
	svc, _ := newTestServices(t,
		WithScoresCSV(scores),
		WithConfigTOML(`
[usage]
backend = "codexbar"

[providers.cursor]
enabled = true

[providers.antigravity]
enabled = true

[providers.opencode]
enabled = true
`),
	)
	stubCatalogRepoFromCache(t, svc)
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{
		{Provider: "opencode", ModelID: "kimi-k3", Name: "Kimi K3", EffortLevels: []string{"max"}},
	})

	previous := discoverLiveProviderModels
	discoverLiveProviderModels = func(_ context.Context, provider string) []routing.ModelEntry {
		switch provider {
		case "cursor":
			return []routing.ModelEntry{{ModelID: "gpt-5.6-sol-medium", Name: "GPT-5.6 Sol", Reasoning: []string{"medium"}}}
		case "antigravity":
			return []routing.ModelEntry{{ModelID: "gemini-3.6-flash-high", Name: "Gemini 3.6 Flash", Reasoning: []string{"high"}}}
		default:
			return nil
		}
	}
	t.Cleanup(func() { discoverLiveProviderModels = previous })

	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatalf("RefreshRoutes() error = %v", err)
	}

	want := map[string]struct {
		modelID   string
		reasoning string
	}{
		"cursor":      {modelID: "gpt-5.6-sol-medium", reasoning: "medium"},
		"antigravity": {modelID: "gemini-3.6-flash-high", reasoning: "high"},
		"opencode":    {modelID: "kimi-k3", reasoning: "max"},
	}
	for provider, expected := range want {
		found := false
		for _, route := range svc.routes.Routes {
			if route.Provider == provider && route.ModelID == expected.modelID && route.Reasoning == expected.reasoning {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("routes = %#v, missing %s/%s@%s", svc.routes.Routes, provider, expected.modelID, expected.reasoning)
		}
	}
	for provider, expected := range want {
		detail, err := svc.Providers().Detail(context.Background(), provider)
		if err != nil {
			t.Fatalf("Detail(%q) error = %v", provider, err)
		}
		found := false
		for _, model := range detail.Models {
			if model.ModelID == expected.modelID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Detail(%q).Models = %#v, missing %s", provider, detail.Models, expected.modelID)
		}
	}
}

func TestRefreshRoutesLiveDiscoveryFailureIsProviderLocal(t *testing.T) {
	const scores = "model,reasoning,intelligence_index_score,time_per_intelligence_index_task_seconds_score,cost_per_intelligence_index_task_usd_score,median_end_to_end_response_time_seconds_score,artificial_analysis_coding_index_score,artificial_analysis_agentic_index_score\n" +
		"Gemini 3.6 Flash,high,100,100,100,100,100,100\n" +
		"Kimi K3,max,100,100,100,100,100,100\n"
	svc, _ := newTestServices(t,
		WithScoresCSV(scores),
		WithConfigTOML(`
[usage]
backend = "codexbar"

[providers.cursor]
enabled = true

[providers.antigravity]
enabled = true

[providers.opencode]
enabled = true
`),
	)
	stubCatalogRepoFromCache(t, svc)
	seedModelsDevCache(t, svc, []modelsdev.ProviderModel{
		{Provider: "opencode", ModelID: "kimi-k3", Name: "Kimi K3", EffortLevels: []string{"max"}},
	})

	previous := discoverLiveProviderModels
	discoverLiveProviderModels = func(_ context.Context, provider string) []routing.ModelEntry {
		if provider == "antigravity" {
			return []routing.ModelEntry{{ModelID: "gemini-3.6-flash-high", Name: "Gemini 3.6 Flash", Reasoning: []string{"high"}}}
		}
		return nil
	}
	t.Cleanup(func() { discoverLiveProviderModels = previous })

	if err := svc.Providers().RefreshRoutes(context.Background()); err != nil {
		t.Fatalf("RefreshRoutes() error = %v", err)
	}
	for _, expected := range []struct {
		provider string
		modelID  string
	}{
		{provider: "antigravity", modelID: "gemini-3.6-flash-high"},
		{provider: "opencode", modelID: "kimi-k3"},
	} {
		found := false
		for _, route := range svc.routes.Routes {
			if route.Provider == expected.provider && route.ModelID == expected.modelID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("routes = %#v, missing %s/%s", svc.routes.Routes, expected.provider, expected.modelID)
		}
	}
	for _, route := range svc.routes.Routes {
		if route.Provider == "cursor" {
			t.Errorf("routes = %#v, cursor discovery failure must not synthesize a live route", svc.routes.Routes)
			break
		}
	}
}
