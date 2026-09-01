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
	if err != nil {
		t.Fatal(err)
	}
	def := config.DefaultGUIConfig()
	authDef := config.DefaultAuthConfig()
	want := guiDTO(def, authDef, svc.paths.UserConfigFile, "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("defaults = %#v, want %#v", got, want)
	}
	in := GUISettings{Layout: "list", DefaultTab: "sliders", WeightControl: "bar", Holds: 10, Shortcut: "cmd+shift+m", ShowMenuBarIcon: false, LaunchAtLogin: true, CopyCommandInstead: true, ClosePopoverAfterLaunch: false, AutoUpdate: false, AutoUpdateFrequency: "weekly", MCPServer: true, ClaudeMDHint: true, ShellAlias: true, UseKeychain: false, ConfigPath: "/evil"}
	if err := svc.Settings().Set(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Settings().Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	in.ConfigPath = svc.paths.UserConfigFile
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round trip = %#v, want %#v", got, in)
	}
	data, err := os.ReadFile(svc.paths.UserConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, key := range []string{"[gui]", "layout = \"list\"", "holds = 10", "[auth]", "use_keychain = false", "[other]", "keep = \"yes\""} {
		if !strings.Contains(text, key) {
			t.Errorf("config missing %q:\n%s", key, text)
		}
	}
	if len(rec.Events()) != 1 || rec.Events()[0].Event != EventSettingsChanged {
		t.Fatalf("events = %#v", rec.Events())
	}
	if !reflect.DeepEqual(rec.Events()[0].Payload, in) {
		t.Fatalf("payload = %#v, want %#v", rec.Events()[0].Payload, in)
	}
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
			case "layout":
				in.Layout = tc.field
			case "weight":
				in.WeightControl = tc.field
			case "holds":
				in.Holds = 4
			case "shortcut":
				in.Shortcut = tc.field
			case "frequency":
				in.AutoUpdateFrequency = tc.field
			}
			err := svc.Settings().Set(context.Background(), in)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
			if len(rec.Events()) != 0 {
				t.Fatalf("events = %#v", rec.Events())
			}
		})
	}
	t.Run("order", func(t *testing.T) {
		svc, _ := newTestServices(t)
		in := base
		in.Layout = "bad"
		in.Holds = 4
		err := svc.Settings().Set(context.Background(), in)
		if err == nil || !strings.Contains(err.Error(), "layout must") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestSettingsSnippetsPinned(t *testing.T) {
	svc, _ := newTestServices(t)
	got := ShellSnippets{Alias: shellAlias, ClaudeMD: shellClaudeMD}
	if got.Alias != "alias wm='which-model pick --profile'" {
		t.Fatal("alias changed")
	}
	if strings.Count(got.ClaudeMD, "\n") != 2 {
		t.Fatal("ClaudeMD must contain exactly three lines")
	}
	_ = svc
}

func TestSettingsShellSnippetsFullStrategyConfig(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML(`[strategy]
default = "priority"
default_profile = "review"
tier1_share = 80
tier2_share = 20
`))

	got, err := svc.Settings().ShellSnippets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.Preview, "$ wm review  →") {
		t.Errorf("preview = %q, want configured review profile", got.Preview)
	}
}

func TestSettingsShellSnippetsRejectsUnknownStrategyKey(t *testing.T) {
	svc, _ := newTestServices(t, WithConfigTOML("[strategy]\nunknown = true\n"))

	_, err := svc.Settings().ShellSnippets(context.Background())
	if err == nil {
		t.Fatal("ShellSnippets error = nil, want strict strategy config error")
	}
	if dto := toErrorDTO(err); dto.Code != "validation_failed" {
		t.Errorf("error code = %q, want validation_failed", dto.Code)
	}
}

// default_tab postdates the GUISettings DTO, so a client written before it (or
// a config saved by one) sends "". That means "unset" and must normalise to the
// shipped default, not fail validation — while a genuinely wrong value still
// errors, and after every other field so the documented order is untouched.
func TestSettingsDefaultTabNormalisation(t *testing.T) {
	svc, _ := newTestServices(t)
	ctx := context.Background()
	base := GUISettings{
		Layout: "carousel", WeightControl: "slider", Holds: 5, Shortcut: "alt+space",
		AutoUpdateFrequency: "daily",
	}

	if err := svc.Settings().Set(ctx, base); err != nil {
		t.Fatalf("Set with empty default_tab: %v", err)
	}
	got, err := svc.Settings().Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultTab != "profiles" {
		t.Errorf("default_tab = %q, want the shipped default %q", got.DefaultTab, "profiles")
	}

	// Errors cross the service boundary as ErrorDTO (toErrorDTO), so this
	// package asserts on the rendered message, as the sibling cases do.
	bad := base
	bad.DefaultTab = "elsewhere"
	want := `validation_failed: validation failed: gui: default_tab must be "profiles" or "sliders", got "elsewhere"`
	if err := svc.Settings().Set(ctx, bad); err == nil || err.Error() != want {
		t.Errorf("Set with bad default_tab err = %v, want %q", err, want)
	}
}
