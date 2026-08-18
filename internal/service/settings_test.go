package service

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/config"
)

func TestSettingsDefaultsAndRoundTrip(t *testing.T) {
	svc, rec := newTestServices(t, WithConfigTOML("[other]\nkeep = \"yes\"\n"))
	got, err := svc.Settings().Get(context.Background())
	if err != nil { t.Fatal(err) }
	def := config.DefaultGUIConfig()
	want := guiDTO(def, svc.paths.UserConfigFile)
	if !reflect.DeepEqual(got, want) { t.Fatalf("defaults = %#v, want %#v", got, want) }
	in := GUISettings{Layout: "list", WeightControl: "bar", Holds: 10, Shortcut: "cmd+shift+m", ShowMenuBarIcon: false, LaunchAtLogin: true, CopyCommandInstead: true, ClosePopoverAfterLaunch: false, AutoUpdate: false, AutoUpdateFrequency: "weekly", MCPServer: true, ClaudeMDHint: true, ShellAlias: true, ConfigPath: "/evil"}
	if err := svc.Settings().Set(context.Background(), in); err != nil { t.Fatal(err) }
	got, err = svc.Settings().Get(context.Background())
	if err != nil { t.Fatal(err) }
	in.ConfigPath = svc.paths.UserConfigFile
	if !reflect.DeepEqual(got, in) { t.Fatalf("round trip = %#v, want %#v", got, in) }
	data, err := os.ReadFile(svc.paths.UserConfigFile)
	if err != nil { t.Fatal(err) }
	text := string(data)
	for _, key := range []string{"[gui]", "layout = \"list\"", "holds = 10", "[other]", "keep = \"yes\""} {
		if !strings.Contains(text, key) { t.Errorf("config missing %q:\n%s", key, text) }
	}
	if len(rec.Events()) != 1 || rec.Events()[0].Event != EventSettingsChanged { t.Fatalf("events = %#v", rec.Events()) }
	if !reflect.DeepEqual(rec.Events()[0].Payload, in) { t.Fatalf("payload = %#v, want %#v", rec.Events()[0].Payload, in) }
}

func TestSettingsValidationOrder(t *testing.T) {
	base := GUISettings{Layout: "carousel", WeightControl: "slider", Holds: 5, Shortcut: "alt+space", AutoUpdateFrequency: "daily"}
	cases := []struct{ name, field, want string }{
		{"layout", "bad", `validation_failed: validation failed: gui: layout must be "carousel" or "list", got "bad"`},
		{"weight", "bad", `validation_failed: validation failed: gui: weight_control must be "step", "bar", or "slider", got "bad"`},
		{"holds", "bad", "validation_failed: validation failed: gui: holds must be 3, 5, or 10, got 4"},
		{"shortcut", "bad", `validation_failed: validation failed: gui: shortcut must be "alt+space", "ctrl+space", or "cmd+shift+m", got "bad"`},
		{"frequency", "bad", `validation_failed: validation failed: gui: auto_update_frequency must be "hourly", "daily", "weekly", or "monthly", got "bad"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, rec := newTestServices(t)
			in := base
			switch tc.name {
			case "layout": in.Layout = tc.field
			case "weight": in.WeightControl = tc.field
			case "holds": in.Holds = 4
			case "shortcut": in.Shortcut = tc.field
			case "frequency": in.AutoUpdateFrequency = tc.field
			}
			err := svc.Settings().Set(context.Background(), in)
			if err == nil || err.Error() != tc.want { t.Fatalf("error = %v, want %q", err, tc.want) }
			if len(rec.Events()) != 0 { t.Fatalf("events = %#v", rec.Events()) }
		})
	}
	 t.Run("order", func(t *testing.T) {
		svc, _ := newTestServices(t)
		in := base; in.Layout = "bad"; in.Holds = 4
		err := svc.Settings().Set(context.Background(), in)
		if err == nil || !strings.Contains(err.Error(), "layout must") { t.Fatalf("error = %v", err) }
	})
}

func TestSettingsSnippetsPinned(t *testing.T) {
	svc, _ := newTestServices(t)
	got := ShellSnippets{Alias: shellAlias, ClaudeMD: shellClaudeMD}
	if got.Alias != "alias wm='which-model pick --profile'" { t.Fatal("alias changed") }
	if strings.Count(got.ClaudeMD, "\n") != 2 { t.Fatal("ClaudeMD must contain exactly three lines") }
	_ = svc
}
