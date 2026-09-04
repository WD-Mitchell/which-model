package whichmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/catalog/publish"
	"github.com/WD-Mitchell/which-model/internal/config"
)

// workflowRepo builds a hermetic fake repo: temp dir with .git/ and a
// which-model.toml at the repo root carrying the given [catalog.publish]
// body ("" = defaults only).
func workflowRepo(t *testing.T, publishBody string) (repo, configPath string) {
	t.Helper()
	repo = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	configPath = filepath.Join(repo, "which-model.toml")
	body := "[catalog.publish]\n" + publishBody
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	t.Chdir(repo)
	Global = GlobalFlags{}
	t.Cleanup(func() { Global = GlobalFlags{} })
	return repo, configPath
}

// loadedPC reproduces the exact PublishConfig the CLI loads for configPath.
func loadedPC(t *testing.T, configPath string) *publish.PublishConfig {
	t.Helper()
	cfg, err := config.Load(config.LoadOptions{Path: configPath})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	pc, err := publish.Load(cfg)
	if err != nil {
		t.Fatalf("publish.Load() error = %v", err)
	}
	return pc
}

func TestCatalogWorkflowWrite(t *testing.T) {
	_, configPath := workflowRepo(t, "")
	code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--config", configPath})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr)
	}
	want := "wrote .github/workflows/refresh-model-data.yml (schedule=\"0 6 * * *\", branches=[main], mode=pull-request)\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	onDisk, err := os.ReadFile(".github/workflows/refresh-model-data.yml")
	if err != nil {
		t.Fatalf("ReadFile(workflow) error = %v", err)
	}
	rendered, err := publish.Render(loadedPC(t, configPath))
	if err != nil {
		t.Fatalf("publish.Render() error = %v", err)
	}
	if string(onDisk) != string(rendered) {
		t.Error("written workflow != publish.Render(pc)")
	}
}

func TestCatalogWorkflowCheckInSync(t *testing.T) {
	_, configPath := workflowRepo(t, "")
	if code, _, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--config", configPath}); code != 0 {
		t.Fatalf("write exit = %d; stderr=%s", code, stderr)
	}
	code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--check", "--config", configPath})
	if code != 0 {
		t.Fatalf("check exit = %d, want 0; stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestCatalogWorkflowCheckDrift(t *testing.T) {
	_, configPath := workflowRepo(t, "")
	if code, _, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--config", configPath}); code != 0 {
		t.Fatalf("write exit = %d; stderr=%s", code, stderr)
	}
	f, err := os.OpenFile(".github/workflows/refresh-model-data.yml", os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		t.Fatalf("append error = %v", err)
	}
	f.Close()

	code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--check", "--config", configPath})
	if code != 1 {
		t.Fatalf("check exit = %d, want 1; stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--- ") || !strings.Contains(stderr, "+++ ") {
		t.Errorf("stderr = %q, want ---/+++ diff headers", stderr)
	}
}

func TestCatalogWorkflowCheckMissing(t *testing.T) {
	_, configPath := workflowRepo(t, "")
	code, _, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--check", "--config", configPath})
	if code != 1 {
		t.Fatalf("check exit = %d, want 1; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "is missing") {
		t.Errorf("stderr = %q, want is missing", stderr)
	}
}

func TestCatalogWorkflowWriteAndCheckMutuallyExclusive(t *testing.T) {
	_, configPath := workflowRepo(t, "")
	code, _, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--check", "--config", configPath})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("stderr = %q, want mutually exclusive", stderr)
	}
}

func TestCatalogWorkflowBadCronConfig(t *testing.T) {
	_, configPath := workflowRepo(t, "schedule = \"bad cron\"\n")
	code, _, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--config", configPath})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr=%s", code, stderr)
	}
	if !strings.Contains(stderr, "catalog.publish.schedule") {
		t.Errorf("stderr = %q, want catalog.publish.schedule", stderr)
	}
}

func TestCatalogWorkflowDisabled(t *testing.T) {
	_, configPath := workflowRepo(t, "enabled = false\n")
	code, stdout, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--config", configPath})
	if code != 0 {
		t.Fatalf("write(absent) exit = %d; stderr=%s", code, stderr)
	}
	if stdout != "no workflow file present (catalog.publish.enabled = false)\n" {
		t.Errorf("stdout = %q", stdout)
	}

	if err := os.WriteFile(".github/workflows/refresh-model-data.yml", []byte("stale\n"), 0o644); err != nil {
		if err := os.MkdirAll(".github/workflows", 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(".github/workflows/refresh-model-data.yml", []byte("stale\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(stale) error = %v", err)
		}
	}
	code, stdout, stderr = captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--config", configPath})
	if code != 0 {
		t.Fatalf("write(present) exit = %d; stderr=%s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "removed ") || !strings.HasSuffix(stdout, "(catalog.publish.enabled = false)\n") {
		t.Errorf("stdout = %q, want removed ... (catalog.publish.enabled = false)", stdout)
	}
	if _, err := os.Stat(".github/workflows/refresh-model-data.yml"); !os.IsNotExist(err) {
		t.Errorf("stale workflow not removed (stat err = %v)", err)
	}
}

func TestCatalogWorkflowOutOverride(t *testing.T) {
	_, configPath := workflowRepo(t, "")
	custom := filepath.Join(t.TempDir(), "custom.yml")
	code, _, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--write", "--out", custom, "--config", configPath})
	if code != 0 {
		t.Fatalf("write exit = %d; stderr=%s", code, stderr)
	}
	if _, err := os.Stat(custom); err != nil {
		t.Fatalf("custom path not written: %v", err)
	}
	if _, err := os.Stat(".github/workflows/refresh-model-data.yml"); !os.IsNotExist(err) {
		t.Errorf("default path written despite --out (stat err = %v)", err)
	}
	code, _, stderr = captureExecuteFresh(t, []string{"catalog", "workflow", "--check", "--out", custom, "--config", configPath})
	if code != 0 {
		t.Fatalf("check --out exit = %d; stderr=%s", code, stderr)
	}
}

func TestCatalogWorkflowCustomRawPathAndSiblings(t *testing.T) {
	_, path := workflowRepo(t, "enabled = true\n")
	if err := os.WriteFile(path, []byte("[catalog]\nraw_csv_path = \"custom raw.csv\"\nscores_csv_path = \"custom scores.csv\"\ncache_ttl = \"12h\"\n[catalog.publish]\nenabled = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := captureExecuteFresh(t, []string{"catalog", "workflow", "--config", path})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	data, err := os.ReadFile(".github/workflows/refresh-model-data.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "custom raw.csv") {
		t.Fatalf("configured staging path missing: %s", data)
	}
}
