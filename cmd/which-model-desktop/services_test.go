package main

import (
	"encoding/json"
	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/service"
	"os"
	"path/filepath"
	"testing"
)

func TestProfilesBindingStructuredErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.toml"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.LoadOptions{Path: filepath.Join(root, "config.toml")})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewEmpty(config.Paths{UserConfigFile: filepath.Join(root, "config.toml"), StateDir: root, CacheDir: root, ConfigDir: root}, cfg, nil)
	api := ProfilesAPI{svc: svc}
	missing, err := api.Get("missing")
	_ = missing
	for _, tc := range []struct {
		err  error
		code string
	}{
		{err, "not_found"},
		{api.Save(service.ProfileDetail{Slug: "bad-slug"}), "validation_failed"},
	} {
		data, e := json.Marshal(tc.err)
		if e != nil {
			t.Fatal(e)
		}
		var dto service.ErrorDTO
		if e = json.Unmarshal(data, &dto); e != nil {
			t.Fatal(e)
		}
		if dto.Code != tc.code || dto.Message == "" {
			t.Fatalf("JSON=%s want %s", data, tc.code)
		}
	}
	if bindingError(nil) != nil {
		t.Fatal("nil became error")
	}
}
