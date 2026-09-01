package service

import (
	"context"
	"fmt"
	"os"

	"github.com/WD-Mitchell/which-model/internal/config"
)

// SettingsService is the settings facet of Services.
type SettingsService struct{ s *Services }

// Settings returns the settings facet.
func (s *Services) Settings() *SettingsService { return &SettingsService{s: s} }

// Get returns the current GUI settings with the resolved config path.
func (g *SettingsService) Get(ctx context.Context) (GUISettings, error) {
	if err := ctx.Err(); err != nil {
		return GUISettings{}, toErrorDTO(err)
	}
	g.s.mu.RLock()
	defer g.s.mu.RUnlock()
	gui, err := g.s.cfg.LoadGUI()
	if err != nil {
		return GUISettings{}, toErrorDTO(err)
	}
	auth, err := g.s.cfg.LoadAuth()
	if err != nil {
		return GUISettings{}, toErrorDTO(err)
	}
	return guiDTO(gui, auth, g.s.paths.UserConfigFile), nil
}

// Set validates and atomically replaces the complete GUI section.
func (g *SettingsService) Set(ctx context.Context, in GUISettings) error {
	if err := ctx.Err(); err != nil {
		return toErrorDTO(err)
	}
	in = normaliseGUISettings(in)
	if err := validateGUISettings(in); err != nil {
		return toErrorDTO(err)
	}
	g.s.mu.Lock()
	// Marshal/decode gives us an independent config document, preserving all
	// unknown sections and values while leaving the live config untouched until
	// persistence succeeds.
	copyCfg, cleanup, err := cloneConfig(g.s.cfg)
	if err == nil {
		err = copyCfg.SetGUI(guiConfig(in))
	}
	if err == nil {
		err = copyCfg.SetAuth(config.AuthConfig{UseKeychain: in.UseKeychain})
	}
	var data []byte
	if err == nil {
		data, err = copyCfg.MarshalTOML()
	}
	if err == nil {
		err = config.AtomicWriteFile(g.s.paths.UserConfigFile, data)
	}
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		g.s.mu.Unlock()
		return toErrorDTO(err)
	}
	g.s.cfg = copyCfg
	g.s.mu.Unlock()

	payload := in
	payload.ConfigPath = g.s.paths.UserConfigFile
	g.s.emit(EventSettingsChanged, payload)
	return nil
}

const shellAlias = "alias wm='which-model pick --profile'"
const shellClaudeMD = "## Model selection\nBefore delegating work, run `wm <profile>` to print the best model id for that task profile.\n`wm` is an alias for `which-model pick --profile`; profiles live in which-model's config.toml."

// ShellSnippets returns pinned setup snippets and a live ranking preview.
func (g *SettingsService) ShellSnippets(ctx context.Context) (ShellSnippets, error) {
	if err := ctx.Err(); err != nil {
		return ShellSnippets{}, toErrorDTO(err)
	}
	slug := "balanced_implementation"
	g.s.mu.RLock()
	var strategy struct {
		DefaultProfile string `toml:"default_profile"`
	}
	if err := g.s.cfg.UnmarshalKey("strategy", &strategy); err == nil && strategy.DefaultProfile != "" {
		slug = strategy.DefaultProfile
	}
	g.s.mu.RUnlock()
	out := ShellSnippets{Alias: shellAlias, ClaudeMD: shellClaudeMD, Preview: "$ wm " + slug + "  →  no route"}
	resp, err := g.s.Rank(ctx, RankRequest{ProfileSlug: slug, Holds: 3})
	if err == nil && len(resp.Candidates) > 0 {
		c := resp.Candidates[0]
		out.Preview = fmt.Sprintf("$ wm %s  →  %s  (%s)", slug, c.ModelID, c.Provider)
	}
	return out, nil
}

func guiDTO(g config.GUIConfig, auth config.AuthConfig, path string) GUISettings {
	return GUISettings{Layout: g.Layout, DefaultTab: g.DefaultTab, WeightControl: g.WeightControl, Holds: g.Holds, Shortcut: g.Shortcut, ShowMenuBarIcon: g.ShowMenuBarIcon, LaunchAtLogin: g.LaunchAtLogin, CopyCommandInstead: g.CopyCommandInstead, ClosePopoverAfterLaunch: g.ClosePopoverAfterLaunch, AutoUpdate: g.AutoUpdate, AutoUpdateFrequency: g.AutoUpdateFrequency, MCPServer: g.MCPServer, ClaudeMDHint: g.ClaudeMDHint, ShellAlias: g.ShellAlias, UseKeychain: auth.UseKeychain, ConfigPath: path}
}

func guiConfig(g GUISettings) config.GUIConfig {
	return config.GUIConfig{Layout: g.Layout, DefaultTab: g.DefaultTab, WeightControl: g.WeightControl, Holds: g.Holds, Shortcut: g.Shortcut, ShowMenuBarIcon: g.ShowMenuBarIcon, LaunchAtLogin: g.LaunchAtLogin, CopyCommandInstead: g.CopyCommandInstead, ClosePopoverAfterLaunch: g.ClosePopoverAfterLaunch, AutoUpdate: g.AutoUpdate, AutoUpdateFrequency: g.AutoUpdateFrequency, MCPServer: g.MCPServer, ClaudeMDHint: g.ClaudeMDHint, ShellAlias: g.ShellAlias}
}

func validateGUISettings(g GUISettings) error {
	if g.Layout != "carousel" && g.Layout != "list" {
		return fmt.Errorf("%w: gui: layout must be \"carousel\" or \"list\", got %q", errValidation, g.Layout)
	}
	if g.WeightControl != "step" && g.WeightControl != "bar" && g.WeightControl != "slider" {
		return fmt.Errorf("%w: gui: weight_control must be \"step\", \"bar\", or \"slider\", got %q", errValidation, g.WeightControl)
	}
	if g.Holds != 3 && g.Holds != 5 && g.Holds != 10 {
		return fmt.Errorf("%w: gui: holds must be 3, 5, or 10, got %d", errValidation, g.Holds)
	}
	if g.Shortcut != "alt+space" && g.Shortcut != "ctrl+space" && g.Shortcut != "cmd+shift+m" {
		return fmt.Errorf("%w: gui: shortcut must be \"alt+space\", \"ctrl+space\", or \"cmd+shift+m\", got %q", errValidation, g.Shortcut)
	}
	if g.AutoUpdateFrequency != "hourly" && g.AutoUpdateFrequency != "daily" && g.AutoUpdateFrequency != "weekly" && g.AutoUpdateFrequency != "monthly" {
		return fmt.Errorf("%w: gui: auto_update_frequency must be \"hourly\", \"daily\", \"weekly\", or \"monthly\", got %q", errValidation, g.AutoUpdateFrequency)
	}
	// Checked last so the existing field order in the error path is untouched.
	// Empty is not an error: it is normalised to the shipped default by
	// normaliseGUISettings before this runs, so only a WRONG value reaches here.
	if g.DefaultTab != "profiles" && g.DefaultTab != "sliders" {
		return fmt.Errorf("%w: gui: default_tab must be \"profiles\" or \"sliders\", got %q", errValidation, g.DefaultTab)
	}
	return nil
}

// normaliseGUISettings fills fields a caller may legitimately omit. default_tab
// postdates the DTO, so a client (or a stored config) written before it sends
// "" — which means "unset", not "invalid".
func normaliseGUISettings(g GUISettings) GUISettings {
	if g.DefaultTab == "" {
		g.DefaultTab = "profiles"
	}
	return g
}

// cloneConfig creates an independent config using the canonical TOML form.
func cloneConfig(src *config.Config) (*config.Config, func(), error) {
	data, err := src.MarshalTOML()
	if err != nil {
		return nil, nil, err
	}
	f, err := os.CreateTemp("", "which-model-config-")
	if err != nil {
		return nil, nil, err
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err = f.Write(data); err == nil {
		err = f.Close()
	} else {
		_ = f.Close()
	}
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	cfg, err := config.LoadFile(name)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return cfg, cleanup, nil
}
