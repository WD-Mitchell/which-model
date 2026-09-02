package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/config"
)

// HarnessService owns the harness registry, install detection, command
// substitution, and launch (B07 SPEC §1). Obtained via Services.Harnesses();
// shares the Services mutex and emit.
type HarnessService struct{ s *Services }

// Harnesses returns the harness sub-service for s.
func (s *Services) Harnesses() *HarnessService { return &HarnessService{s: s} }

// harnessSeed is one builtin harness written into config on first List
// (B07 SPEC §2.2; CONTRACTS §3).
type harnessSeed struct {
	slug      string
	name      string
	command   string
	providers []string
}

// harnessSeeds is the exact CONTRACTS §3 builtin seed table. Order matters
// only for determinism; SetHarness writes each subtable, config sorts output.
var harnessSeeds = []harnessSeed{
	{"aider", "Aider", "aider --model {model_id}", []string{"claude", "codex", "copilot", "cursor"}},
	{"claude", "Claude Code", "claude --model {model_id} --reasoning {reasoning}", []string{"claude", "codex", "copilot"}},
	{"codex", "Codex CLI", "codex -m {model_id} -c reasoning={reasoning}", []string{"codex", "copilot"}},
	{"copilot", "Copilot CLI", "copilot --model {model_id}", []string{"copilot", "cursor"}},
	{"cursor", "Cursor", "cursor --model {model_id}", []string{"cursor"}},
	{"goose", "Goose", "goose session --model {model_id}", []string{"claude", "codex", "copilot"}},
	{"windsurf", "Windsurf", "windsurf --model {model_id}", []string{"claude", "codex", "copilot", "cursor"}},
}

// harnessTokenRe matches any {token} placeholder left after substitution;
// the first match names an unresolved token (CONTRACTS §5.2, §6 #4).
var harnessTokenRe = regexp.MustCompile(`\{[a-z0-9_]+\}`)

// List returns every harness in slug-ascending order. On the first call that
// finds [harnesses] with no subtables it seeds the four builtins into config
// (one write, one config:changed event) before answering (SPEC §2.2).
// Installed is recomputed from the live PATH on every call, never persisted
// (SPEC §2.4). Providers is a map over all configured provider ids, true iff
// the id is in the harness's allow-list; ids absent from [providers.*] are
// omitted from the map (SPEC §2.3).
func (h *HarnessService) List(ctx context.Context) ([]HarnessInfo, error) {
	_ = ctx
	if err := h.seedIfEmpty(); err != nil {
		return nil, err
	}
	h.s.mu.RLock()
	defer h.s.mu.RUnlock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		return nil, err
	}
	providerSet := make(map[string]bool, len(h.s.cfg.Providers))
	for id := range h.s.cfg.Providers {
		providerSet[id] = true
	}
	slugs := make([]string, 0, len(harnesses))
	for slug := range harnesses {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	out := make([]HarnessInfo, 0, len(slugs))
	for _, slug := range slugs {
		ht := harnesses[slug]
		inst := installed(ht.Command)
		en := inst
		if ht.Enabled != nil {
			en = *ht.Enabled
		}
		out = append(out, HarnessInfo{
			Slug:      slug,
			Name:      ht.Name,
			Command:   ht.Command,
			Builtin:   ht.Builtin,
			Installed: inst,
			Enabled:   en,
			Providers: providerMap(ht.Providers, providerSet),
		})
	}
	return out, nil
}

// seedIfEmpty writes the four builtin seeds iff [harnesses] has no subtables
// at all (SPEC §2.2). Seeding happens at most once: any non-empty section —
// even a single custom — suppresses it forever. Emits one config:changed for
// the write.
func (h *HarnessService) seedIfEmpty() error {
	h.s.mu.Lock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	if len(harnesses) > 0 {
		h.s.mu.Unlock()
		return nil
	}
	next := *h.s.cfg
	for _, seed := range harnessSeeds {
		if err := next.SetHarness(seed.slug, config.HarnessTOML{
			Name: seed.name, Command: seed.command, Providers: seed.providers, Builtin: true,
		}); err != nil {
			h.s.mu.Unlock()
			return err
		}
	}
	data, err := next.MarshalTOML()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	if err := config.AtomicWriteFile(h.s.paths.UserConfigFile, data); err != nil {
		h.s.mu.Unlock()
		return err
	}
	h.s.cfg = &next
	h.s.mu.Unlock()
	h.s.emit(EventConfigChanged, map[string]string{"section": "harnesses"})
	return nil
}

// Save upserts a harness. Validation order (SPEC §2.5): slug grammar -> name
// -> command -> builtin protection. For an existing builtin, Name/Command
// must equal the stored values or errBuiltinReadonly (only the provider map
// may change). New slugs are created custom regardless of in.Builtin.
// Installed is ignored. Emits config:changed{section:"harnesses"}.
func (h *HarnessService) Save(ctx context.Context, in HarnessInfo) error {
	_ = ctx
	if !providerRe.MatchString(in.Slug) {
		return fmt.Errorf("%w: harness slug %q must match [a-z0-9_]+", errValidation, in.Slug)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: harness name must not be empty", errValidation)
	}
	if in.Command == "" {
		return fmt.Errorf("%w: harness command must not be empty", errValidation)
	}
	h.s.mu.Lock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	stored, exists := harnesses[in.Slug]
	builtin := exists && stored.Builtin
	if builtin && (in.Name != stored.Name || in.Command != stored.Command) {
		h.s.mu.Unlock()
		return fmt.Errorf("%w: harness %q is builtin: name and command are read-only", errBuiltinReadonly, in.Slug)
	}
	next := *h.s.cfg
	if err := next.SetHarness(in.Slug, config.HarnessTOML{
		Name: in.Name, Command: in.Command, Builtin: builtin, Providers: enabledProviders(in.Providers),
	}); err != nil {
		h.s.mu.Unlock()
		return err
	}
	return h.persist(&next)
}

// Delete removes ANY harness, builtin or custom (SPEC Deviations). Unknown
// slug -> errNotFound. Emits config:changed{section:"harnesses"}.
func (h *HarnessService) Delete(ctx context.Context, slug string) error {
	_ = ctx
	h.s.mu.Lock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	if _, ok := harnesses[slug]; !ok {
		h.s.mu.Unlock()
		return fmt.Errorf("%w: harness %q not found", errNotFound, slug)
	}
	next := *h.s.cfg
	next.DeleteHarness(slug)
	return h.persist(&next)
}

// SetProvider toggles one provider in the harness allow-list (idempotent;
// the list is stored sorted, deduplicated). Unknown slug -> errNotFound;
// a provider not configured under [providers.*] -> errValidation. Emits
// config:changed{section:"harnesses"}.
func (h *HarnessService) SetProvider(ctx context.Context, slug, provider string, on bool) error {
	_ = ctx
	h.s.mu.Lock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	stored, ok := harnesses[slug]
	if !ok {
		h.s.mu.Unlock()
		return fmt.Errorf("%w: harness %q not found", errNotFound, slug)
	}
	if _, exists := h.s.cfg.Providers[provider]; !exists {
		h.s.mu.Unlock()
		return fmt.Errorf("%w: unknown provider %q", errValidation, provider)
	}
	next := *h.s.cfg
	if err := next.SetHarness(slug, config.HarnessTOML{
		Name: stored.Name, Command: stored.Command, Builtin: stored.Builtin,
		Providers: toggleProvider(stored.Providers, provider, on),
	}); err != nil {
		h.s.mu.Unlock()
		return err
	}
	return h.persist(&next)
}

// SetAllProviders sets every configured provider on/off for a harness:
// on=true -> allow-list = every configured provider id; on=false -> empty.
// Unknown slug -> errNotFound. Emits config:changed{section:"harnesses"}.
func (h *HarnessService) SetAllProviders(ctx context.Context, slug string, on bool) error {
	_ = ctx
	h.s.mu.Lock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	stored, ok := harnesses[slug]
	if !ok {
		h.s.mu.Unlock()
		return fmt.Errorf("%w: harness %q not found", errNotFound, slug)
	}
	var list []string
	if on {
		list = make([]string, 0, len(h.s.cfg.Providers))
		for id := range h.s.cfg.Providers {
			list = append(list, id)
		}
		sort.Strings(list)
	}
	next := *h.s.cfg
	if err := next.SetHarness(slug, config.HarnessTOML{
		Name: stored.Name, Command: stored.Command, Builtin: stored.Builtin, Providers: list,
	}); err != nil {
		h.s.mu.Unlock()
		return err
	}
	return h.persist(&next)
}

// SetEnabled sets whether a harness is enabled. An explicit setting overrides
// the default auto-detected (installed) state. Unknown slug -> errNotFound.
// Emits config:changed{section:"harnesses"}.
func (h *HarnessService) SetEnabled(ctx context.Context, slug string, on bool) error {
	_ = ctx
	h.s.mu.Lock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	stored, ok := harnesses[slug]
	if !ok {
		h.s.mu.Unlock()
		return fmt.Errorf("%w: harness %q not found", errNotFound, slug)
	}
	next := *h.s.cfg
	if err := next.SetHarness(slug, config.HarnessTOML{
		Name: stored.Name, Command: stored.Command, Builtin: stored.Builtin,
		Providers: stored.Providers, Enabled: &on,
	}); err != nil {
		h.s.mu.Unlock()
		return err
	}
	return h.persist(&next)
}

// persist applies a writer under the already-held write lock: it marshals the
// copy, writes it atomically, swaps it into memory, then (after unlock) emits
// config:changed{section:"harnesses"}. A failed write leaves in-memory state
// untouched and emits nothing.
func (h *HarnessService) persist(next *config.Config) error {
	data, err := next.MarshalTOML()
	if err != nil {
		h.s.mu.Unlock()
		return err
	}
	if err := config.AtomicWriteFile(h.s.paths.UserConfigFile, data); err != nil {
		h.s.mu.Unlock()
		return err
	}
	h.s.cfg = next
	h.s.mu.Unlock()
	h.s.emit(EventConfigChanged, map[string]string{"section": "harnesses"})
	return nil
}

// BuildCommand substitutes {model_id} then {reasoning} via ReplaceAll (a
// template missing either token is valid — the replace no-ops). Afterwards any
// remaining {token} matching \{[a-z0-9_]+\} -> errValidation naming the first
// one. reasoning == "default" substitutes verbatim. Unknown slug ->
// errNotFound. Pure; no side effects.
func (h *HarnessService) BuildCommand(slug, modelID, reasoning string) (string, error) {
	h.s.mu.RLock()
	defer h.s.mu.RUnlock()
	harnesses, err := h.s.cfg.LoadHarnesses()
	if err != nil {
		return "", err
	}
	stored, ok := harnesses[slug]
	if !ok {
		return "", fmt.Errorf("%w: harness %q not found", errNotFound, slug)
	}
	cmd := strings.ReplaceAll(stored.Command, "{model_id}", modelID)
	cmd = strings.ReplaceAll(cmd, "{reasoning}", reasoning)
	if m := harnessTokenRe.FindString(cmd); m != "" {
		return "", fmt.Errorf("%w: harness %q: unresolved template token %q", errValidation, slug, m)
	}
	return cmd, nil
}

// Launch parses the route key, builds the substituted command, then either
// returns it for copying ([gui].copy_command_instead) or spawns it detached
// via userShell() -lc with launchSysProcAttr(), stdout/stderr appended to
// <StateDir>/launch.log. Start failure -> errLaunchFailed. On success (both
// modes) the pick is recorded via the recordPick seam; a record failure is
// logged, not returned (SPEC §2.9–2.10).
func (h *HarnessService) Launch(ctx context.Context, slug, routeKey, profileSlug string) (LaunchResult, error) {
	_, modelID, reasoning, err := ParseRouteKey(routeKey)
	if err != nil {
		return LaunchResult{}, err
	}
	cmd, err := h.BuildCommand(slug, modelID, reasoning)
	if err != nil {
		return LaunchResult{}, err
	}

	h.s.mu.RLock()
	gui, gerr := h.s.cfg.LoadGUI()
	copyMode := false
	if gerr == nil {
		copyMode = gui.CopyCommandInstead
	}
	h.s.mu.RUnlock()

	if copyMode {
		h.recordPick(ctx, profileSlug, routeKey)
		return LaunchResult{Copied: true, Command: cmd}, nil
	}

	logFile, err := os.OpenFile(filepath.Join(h.s.paths.StateDir, "launch.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return LaunchResult{}, err
	}
	proc := exec.Command(userShell(), "-lc", cmd)
	proc.SysProcAttr = launchSysProcAttr()
	proc.Stdin = nil
	proc.Stdout = logFile
	proc.Stderr = logFile
	if err := proc.Start(); err != nil {
		logFile.Close()
		return LaunchResult{}, fmt.Errorf("%w: launch %q: %v", errLaunchFailed, slug, err)
	}
	logFile.Close()
	proc.Process.Release() // never waited on (SPEC §2.9.4)
	h.recordPick(ctx, profileSlug, routeKey)
	return LaunchResult{Copied: false, Command: cmd}, nil
}

// recordPick invokes the recordPick seam, logging (never returning) a failure
// so a running harness launch is not failed by pick bookkeeping.
func (h *HarnessService) recordPick(ctx context.Context, profileSlug, routeKey string) {
	if h.s.recordPick == nil {
		return
	}
	if err := h.s.recordPick(ctx, profileSlug, routeKey); err != nil {
		log.Printf("harness: record pick for %q: %v", routeKey, err)
	}
}

// userShell returns the login shell for launching harness commands: $SHELL,
// falling back to /bin/sh (SPEC §2.9.4).
func userShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

// installed reports whether the command's argv0 resolves on the live PATH.
// Empty/whitespace-only commands are never installed (SPEC §2.4).
func installed(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	_, err := exec.LookPath(fields[0])
	return err == nil
}

// providerMap builds the DTO allow-map: every configured provider id maps to
// whether it appears in the harness's list; list ids absent from the provider
// set are dropped (SPEC §2.3).
func providerMap(list []string, providerSet map[string]bool) map[string]bool {
	out := make(map[string]bool, len(providerSet))
	for id := range providerSet {
		out[id] = contains(list, id)
	}
	return out
}

// enabledProviders returns the sorted ids whose allow-map value is true (the
// persisted list shape; stored sorted ascending).
func enabledProviders(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for id, on := range m {
		if on {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// toggleProvider adds or removes provider, keeping the list sorted and
// deduplicated (SPEC §2.7); an empty result stays non-nil so SetHarness
// persists an empty allow-list.
func toggleProvider(list []string, provider string, on bool) []string {
	out := make([]string, 0, len(list)+1)
	for _, p := range list {
		if p != provider {
			out = append(out, p)
		}
	}
	if on {
		out = append(out, provider)
	}
	sort.Strings(out)
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
