package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/catalog/identity"
	"github.com/WD-Mitchell/which-model/internal/routing"
)

const (
	providerModelCommandTimeout = 15 * time.Second
	maxProviderModelOutputBytes = 1 << 20
)

var (
	providerModelIDPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	errProviderModelOutput  = errors.New("provider model command returned unusable output")
	runProviderModelCommand = runProviderModelCommandDefault
)

type providerModelCommand struct {
	binary string
	args   []string
}

func discoverLiveProviderModelsDefault(ctx context.Context, provider string) []routing.ModelEntry {
	var commands []providerModelCommand
	var parse func(string) ([]routing.ModelEntry, error)
	switch provider {
	case "cursor":
		commands = []providerModelCommand{{binary: "cursor-agent", args: []string{"--list-models"}}}
		parse = parseCursorModelList
	case "antigravity":
		commands = []providerModelCommand{
			{binary: "agy", args: []string{"models"}},
			{binary: "antigravity", args: []string{"models"}},
		}
		parse = parseAntigravityModelList
	default:
		return nil
	}

	for _, command := range commands {
		output, err := runProviderModelCommand(ctx, command.binary, command.args...)
		if err != nil {
			continue
		}
		models, err := parse(string(output))
		if err == nil {
			return models
		}
	}
	return nil
}

func runProviderModelCommandDefault(ctx context.Context, binary string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx := ctx
	cancel := func() {}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > providerModelCommandTimeout {
		runCtx, cancel = context.WithTimeout(ctx, providerModelCommandTimeout)
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, binary, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	var output providerModelOutput
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil || output.tooLarge || output.Len() == 0 {
		return nil, errProviderModelOutput
	}
	return output.Bytes(), nil
}

type providerModelOutput struct {
	bytes.Buffer
	tooLarge bool
}

func (w *providerModelOutput) Write(data []byte) (int, error) {
	remaining := maxProviderModelOutputBytes - w.Len()
	if remaining <= 0 {
		w.tooLarge = true
		return len(data), nil
	}
	if len(data) > remaining {
		_, _ = w.Buffer.Write(data[:remaining])
		w.tooLarge = true
		return len(data), nil
	}
	_, _ = w.Buffer.Write(data)
	return len(data), nil
}

func parseCursorModelList(output string) ([]routing.ModelEntry, error) {
	return parseProviderModelLines(output, func(line string) (id, name string, skip bool, ok bool) {
		switch {
		case line == "Available models", strings.HasPrefix(line, "Tip:"):
			return "", "", true, true
		}
		id, name, ok = strings.Cut(line, " - ")
		if !ok {
			return "", "", false, false
		}
		if strings.TrimSpace(id) == "auto" {
			return "", "", true, true
		}
		return id, name, false, true
	})
}

func parseAntigravityModelList(output string) ([]routing.ModelEntry, error) {
	return parseProviderModelLines(output, func(line string) (id, name string, skip bool, ok bool) {
		if line == "Fetching available models..." {
			return "", "", true, true
		}
		id, name, ok = strings.Cut(line, "\t")
		return id, name, false, ok
	})
}

func parseProviderModelLines(
	output string,
	parseLine func(string) (id, name string, skip bool, ok bool),
) ([]routing.ModelEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	models := make([]routing.ModelEntry, 0)
	seen := make(map[string]struct{})
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		id, name, skip, ok := parseLine(line)
		if !ok {
			return nil, errProviderModelOutput
		}
		if skip {
			continue
		}
		entry, ok := providerModelEntry(strings.TrimSpace(id), strings.TrimSpace(name))
		if !ok {
			return nil, errProviderModelOutput
		}
		if _, duplicate := seen[entry.ModelID]; duplicate {
			return nil, errProviderModelOutput
		}
		seen[entry.ModelID] = struct{}{}
		models = append(models, entry)
	}
	if scanner.Err() != nil || len(models) == 0 {
		return nil, errProviderModelOutput
	}
	return models, nil
}

func providerModelEntry(id, displayName string) (routing.ModelEntry, bool) {
	if !providerModelIDPattern.MatchString(id) {
		return routing.ModelEntry{}, false
	}
	level, hasLevel := providerModelEffort(id)
	name := normalizeProviderModelName(id, displayName, level)
	if name == "" {
		return routing.ModelEntry{}, false
	}
	entry := routing.ModelEntry{ModelID: id, Name: name}
	if hasLevel {
		entry.Reasoning = []string{level}
	}
	return entry, true
}

func providerModelEffort(id string) (string, bool) {
	base := strings.TrimSuffix(strings.ToLower(id), "-fast")
	if strings.HasSuffix(base, "-extra-high") {
		return "xhigh", true
	}
	for _, suffix := range []struct {
		text  string
		level string
	}{
		{"-minimal", "minimal"},
		{"-low", "low"},
		{"-medium", "medium"},
		{"-high", "high"},
		{"-xhigh", "xhigh"},
		{"-max", "max"},
		{"-none", "default"},
	} {
		if strings.HasSuffix(base, suffix.text) {
			return suffix.level, true
		}
	}
	return "", false
}

func normalizeProviderModelName(id, displayName, level string) string {
	name := identity.CleanModelName(displayName)
	for {
		before := name
		name = trimSuffixFold(name, " Fast")
		name = trimSuffixFold(name, " Thinking")
		switch level {
		case "minimal":
			name = trimSuffixFold(name, " Minimal")
		case "low":
			name = trimSuffixFold(name, " Low")
		case "medium":
			name = trimSuffixFold(name, " Medium")
		case "high":
			name = trimSuffixFold(name, " High")
		case "xhigh":
			name = trimSuffixFold(name, " Extra High")
			name = trimSuffixFold(name, " XHigh")
		case "max":
			name = trimSuffixFold(name, " Maximum")
			name = trimSuffixFold(name, " Max")
		case "default":
			name = trimSuffixFold(name, " None")
		}
		name = trimSuffixFold(name, " 1M")
		if name == before {
			break
		}
	}
	if strings.HasPrefix(strings.ToLower(id), "cursor-") && strings.HasPrefix(strings.ToLower(name), "cursor ") {
		name = strings.TrimSpace(name[len("Cursor "):])
	}
	return name
}

func trimSuffixFold(value, suffix string) string {
	if len(value) < len(suffix) || !strings.EqualFold(value[len(value)-len(suffix):], suffix) {
		return value
	}
	return strings.TrimSpace(value[:len(value)-len(suffix)])
}
