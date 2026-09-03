package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"regexp"
	"sort"
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
	return parseCursorModelLines(output)
}

func parseAntigravityModelList(output string) ([]routing.ModelEntry, error) {
	return parseProviderModelLines(
		output,
		parseAntigravityLine,
		parseAntigravityModelEntry,
	)
}
func parseCursorLine(line string) (id, name string, skip bool, ok bool) {
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
}

func parseAntigravityLine(line string) (id, name string, skip bool, ok bool) {
	if line == "Fetching available models..." {
		return "", "", true, true
	}
	id, name, ok = strings.Cut(line, "\t")
	return id, name, false, ok
}

type providerModelAccumulator struct {
	order      []string
	models     map[string]*accumulatedProviderModel
	seenRawIDs map[string]struct{}
}

type accumulatedProviderModel struct {
	baseID     string
	name       string
	levels     []string
	seenLevels map[string]struct{}
}

func newProviderModelAccumulator() *providerModelAccumulator {
	return &providerModelAccumulator{
		order:      make([]string, 0),
		models:     make(map[string]*accumulatedProviderModel),
		seenRawIDs: make(map[string]struct{}),
	}
}

func (a *providerModelAccumulator) add(rawID, baseID, name, effort string) error {
	if _, duplicate := a.seenRawIDs[rawID]; duplicate {
		return errProviderModelOutput
	}
	a.seenRawIDs[rawID] = struct{}{}

	m, exists := a.models[baseID]
	if !exists {
		m = &accumulatedProviderModel{
			baseID:     baseID,
			name:       name,
			seenLevels: make(map[string]struct{}),
		}
		a.models[baseID] = m
		a.order = append(a.order, baseID)
	} else if m.name == "" && name != "" {
		m.name = name
	}

	if effort != "" {
		if _, hasLevel := m.seenLevels[effort]; !hasLevel {
			m.seenLevels[effort] = struct{}{}
			m.levels = append(m.levels, effort)
		}
	}
	return nil
}

func (a *providerModelAccumulator) entries() []routing.ModelEntry {
	entries := make([]routing.ModelEntry, 0, len(a.order))
	for _, baseID := range a.order {
		m := a.models[baseID]
		sort.SliceStable(m.levels, func(i, j int) bool {
			return reasoningLess(m.levels[i], m.levels[j])
		})
		entry := routing.ModelEntry{
			ModelID: m.baseID,
			Name:    m.name,
		}
		if len(m.levels) > 0 {
			entry.Reasoning = m.levels
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseCursorModelLines(output string) ([]routing.ModelEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	acc := newCursorModelAccumulator()
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		rawID, rawName, skip, ok := parseCursorLine(line)
		if !ok {
			return nil, errProviderModelOutput
		}
		if skip {
			continue
		}
		rawID = strings.TrimSpace(rawID)
		rawName = strings.TrimSpace(rawName)
		if err := acc.add(rawID, rawName); err != nil {
			return nil, err
		}
	}
	if scanner.Err() != nil || len(acc.order) == 0 {
		return nil, errProviderModelOutput
	}
	return acc.entries(), nil
}

type cursorVariantCandidate struct {
	rawID      string
	name       string
	score      int
	maxContext bool // candidate came from a -max context-window row, not an unsuffixed id
}

type cursorModelFamily struct {
	baseID    string
	name      string
	efforts   map[string]*cursorVariantCandidate
	order     []string
	hasEffort bool
}

type cursorModelAccumulator struct {
	order      []string
	families   map[string]*cursorModelFamily
	seenRawIDs map[string]struct{}
}

func newCursorModelAccumulator() *cursorModelAccumulator {
	return &cursorModelAccumulator{
		order:      make([]string, 0),
		families:   make(map[string]*cursorModelFamily),
		seenRawIDs: make(map[string]struct{}),
	}
}

func (a *cursorModelAccumulator) add(rawID, rawName string) error {
	if !providerModelIDPattern.MatchString(rawID) {
		return errProviderModelOutput
	}
	if _, duplicate := a.seenRawIDs[rawID]; duplicate {
		return errProviderModelOutput
	}
	a.seenRawIDs[rawID] = struct{}{}

	baseID, effort, maxCtx, ok := parseCursorModelID(rawID)
	if !ok {
		return errProviderModelOutput
	}
	name := normalizeCursorModelName(baseID, rawName, effort)
	if name == "" {
		return errProviderModelOutput
	}

	fam, exists := a.families[baseID]
	if !exists {
		fam = &cursorModelFamily{
			baseID:  baseID,
			name:    name,
			efforts: make(map[string]*cursorVariantCandidate),
			order:   make([]string, 0),
		}
		a.families[baseID] = fam
		a.order = append(a.order, baseID)
	} else if fam.name == "" && name != "" {
		fam.name = name
	}

	score := cursorRawIDScore(rawID)
	if effort != "" {
		fam.hasEffort = true
	}

	existing, hasEffort := fam.efforts[effort]
	if !hasEffort {
		fam.efforts[effort] = &cursorVariantCandidate{rawID: rawID, name: name, score: score, maxContext: maxCtx}
		fam.order = append(fam.order, effort)
	} else if score > existing.score {
		existing.rawID = rawID
		existing.score = score
		existing.maxContext = maxCtx
		if existing.name == "" && name != "" {
			existing.name = name
		}
	}
	return nil
}

func (a *cursorModelAccumulator) entries() []routing.ModelEntry {
	entries := make([]routing.ModelEntry, 0)
	for _, baseID := range a.order {
		fam := a.families[baseID]
		if fam.hasEffort {
			// Multi-effort model: emit the canonical raw ID for each explicit
			// effort level in canonical ladder order. Context-window -max rows
			// (empty effort, maxContext) do not create separate entries.
			efforts := make([]string, 0, len(fam.efforts))
			for effort := range fam.efforts {
				if effort != "" {
					efforts = append(efforts, effort)
				}
			}
			sort.SliceStable(efforts, func(i, j int) bool {
				return reasoningLess(efforts[i], efforts[j])
			})
			for _, effort := range efforts {
				cand := fam.efforts[effort]
				entries = append(entries, routing.ModelEntry{
					ModelID:   cand.rawID,
					Name:      cand.name,
					Reasoning: []string{effort},
				})
			}
			// An unsuffixed executable ID (empty effort, NOT a -max row) is a
			// distinct advertised route (the provider's default launch target,
			// e.g. gpt-5.3-codex) and must survive alongside effort routes.
			if cand := fam.efforts[""]; cand != nil && !cand.maxContext {
				entries = append(entries, routing.ModelEntry{
					ModelID: cand.rawID,
					Name:    cand.name,
				})
			}
		} else if cand := fam.efforts[""]; cand != nil {
			// Single model with no effort level (e.g. claude-fable-5-max,
			// composer-2.5): emit whichever representative row won scoring.
			entries = append(entries, routing.ModelEntry{
				ModelID: cand.rawID,
				Name:    cand.name,
			})
		}
	}
	return entries
}

func cursorRawIDScore(rawID string) int {
	lower := strings.ToLower(rawID)
	isFast := strings.HasSuffix(lower, "-fast")
	isThinking := strings.Contains(lower, "-thinking")
	switch {
	case !isFast && !isThinking:
		return 4
	case !isFast && isThinking:
		return 3
	case isFast && !isThinking:
		return 2
	default:
		return 1
	}
}

func parseProviderModelLines(
	output string,
	parseLine func(string) (id, name string, skip bool, ok bool),
	normalize func(rawID, rawName string) (baseID, cleanName, effort string, ok bool),
) ([]routing.ModelEntry, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	acc := newProviderModelAccumulator()
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		rawID, rawName, skip, ok := parseLine(line)
		if !ok {
			return nil, errProviderModelOutput
		}
		if skip {
			continue
		}
		rawID = strings.TrimSpace(rawID)
		rawName = strings.TrimSpace(rawName)
		baseID, cleanName, effort, ok := normalize(rawID, rawName)
		if !ok {
			return nil, errProviderModelOutput
		}
		if err := acc.add(rawID, baseID, cleanName, effort); err != nil {
			return nil, err
		}
	}
	if scanner.Err() != nil || len(acc.order) == 0 {
		return nil, errProviderModelOutput
	}
	return acc.entries(), nil
}

func parseCursorModelID(rawID string) (baseID, effort string, maxContext bool, ok bool) {
	lower := strings.ToLower(rawID)
	id := strings.TrimSuffix(lower, "-fast")
	id = strings.TrimSuffix(id, "-thinking")
	if strings.HasSuffix(id, "-max") {
		id = strings.TrimSuffix(id, "-max")
		id = strings.TrimSuffix(id, "-thinking")
		maxContext = true
	}

	level := ""
	if strings.HasSuffix(id, "-extra-high") {
		level = "xhigh"
		id = strings.TrimSuffix(id, "-extra-high")
	} else {
		for _, suffix := range []struct {
			text  string
			level string
		}{
			{"-minimal", "minimal"},
			{"-low", "low"},
			{"-medium", "medium"},
			{"-high", "high"},
			{"-xhigh", "xhigh"},
			{"-none", "default"},
		} {
			if strings.HasSuffix(id, suffix.text) {
				level = suffix.level
				id = strings.TrimSuffix(id, suffix.text)
				break
			}
		}
	}

	id = strings.TrimSuffix(id, "-thinking")
	if strings.HasSuffix(id, "-max") {
		id = strings.TrimSuffix(id, "-max")
		id = strings.TrimSuffix(id, "-thinking")
		maxContext = true
	}

	if id == "" {
		return "", "", false, false
	}
	baseID = rawID[:len(id)]
	return baseID, level, maxContext, true
}
func parseAntigravityModelEntry(id, displayName string) (baseID, name, effort string, ok bool) {
	if !providerModelIDPattern.MatchString(id) {
		return "", "", "", false
	}
	level, _ := providerModelEffort(id)
	cleanName := normalizeAntigravityModelName(id, displayName, level)
	if cleanName == "" {
		return "", "", "", false
	}
	return id, cleanName, level, true
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

func normalizeCursorModelName(id, displayName, level string) string {
	name := identity.CleanModelName(displayName)
	for {
		before := name
		name = trimSuffixFold(name, " Fast")
		name = trimSuffixFold(name, " Thinking")
		name = trimSuffixFold(name, " Max")
		name = trimSuffixFold(name, " Maximum")
		name = trimSuffixFold(name, " 1M")
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
		case "default":
			name = trimSuffixFold(name, " None")
		}
		if name == before {
			break
		}
	}
	if strings.HasPrefix(strings.ToLower(id), "cursor-") && strings.HasPrefix(strings.ToLower(name), "cursor ") {
		name = strings.TrimSpace(name[len("Cursor "):])
	}
	return name
}

func normalizeAntigravityModelName(id, displayName, level string) string {
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
		if name == before {
			break
		}
	}
	return name
}

func trimSuffixFold(value, suffix string) string {
	if len(value) < len(suffix) || !strings.EqualFold(value[len(value)-len(suffix):], suffix) {
		return value
	}
	return strings.TrimSpace(value[:len(value)-len(suffix)])
}
