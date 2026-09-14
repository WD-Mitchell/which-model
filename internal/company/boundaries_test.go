//go:build !nousage

package company_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WD-Mitchell/which-model/internal/company"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/hooks"
	"github.com/WD-Mitchell/which-model/internal/service"
	"github.com/WD-Mitchell/which-model/internal/skills"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/WD-Mitchell/which-model/internal/usage/fetch"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/claude"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codex"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/codexbar"
	"github.com/WD-Mitchell/which-model/internal/usage/provider/cursor"
)

// Runs only in a dedicated native CI job enrolled through the actual fixed OS
// origin. Every called package uses its production policy reader, without seams.
func TestNativeManagedOperationBoundaries(t *testing.T) {
	if os.Getenv("COMPANY_POLICY_LIVE_TEST") != "1" {
		t.Skip("isolated CI enrollment required")
	}
	s, err := company.Load()
	if err != nil || !s.Managed || !s.Required {
		t.Fatal("CI enrollment not active", err)
	}
	ctx := context.Background()
	root := t.TempDir()
	canary := filepath.Join(root, "UNREAD_CREDENTIAL_CANARY")
	refused := func(err error) {
		t.Helper()
		var denied *company.Error
		if !errors.As(err, &denied) || strings.Contains(err.Error(), "CANARY") {
			t.Fatalf("expected policy refusal before effects: %v", err)
		}
	}
	_, err = codex.LoadCredential(canary, canary)
	refused(err)
	_, err = claude.LoadFileCredential(canary, canary, time.Time{})
	refused(err)
	refused(codex.PersistLogin(codex.Tokens{AccessToken: "SYNTHETIC_CANARY"}))
	refused(claude.PersistLogin(claude.Tokens{AccessToken: "SYNTHETIC_CANARY"}))
	_, err = codex.StartDeviceLogin(ctx, "https://invalid.invalid", "CANARY", nil)
	refused(err)
	_, err = claude.StartBrowserLogin()
	refused(err)
	_, err = cursor.StartBrowserLogin(ctx)
	refused(err)
	_, err = codexbar.Fetch(ctx, "codex")
	refused(err)
	t.Setenv("WHICH_MODEL_TEST_CREDENTIAL", "SYNTHETIC_CANARY")
	_, err = (&credential.EnvResolver{Var: "WHICH_MODEL_TEST_CREDENTIAL"}).Resolve(ctx)
	refused(err)
	_, _, err = fetch.FetchAll(ctx, []string{"forbidden"}, fetch.Options{Enabled: map[string]bool{"forbidden": true}, CacheDir: root})
	refused(err)
	_, err = skills.Install("model-selection", skills.TargetGeneric, false, false)
	refused(err)
	_, err = hooks.Install("claude", hooks.Installed(hooks.VariantUsage), root)
	refused(err)
	ran := false
	_, err = hooks.Run("usage-refresh", nil, hooks.Options{Runner: func([]string, io.Writer, io.Writer) int { ran = true; return 0 }})
	refused(err)
	if ran {
		t.Fatal("disabled hook invoked its runner")
	}
	svc := service.NewEmpty(config.Paths{ConfigDir: root, CacheDir: root, StateDir: root}, config.Default(), nil)
	_, err = svc.Harnesses().Launch(ctx, "CANARY", "CANARY", "CANARY")
	refused(err)
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("refused operations wrote files", err, entries)
	}
}
