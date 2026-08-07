package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBoundedFile(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(name string, content []byte, perm os.FileMode) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, perm); err != nil {
			t.Fatal(err)
		}
		// os.Chmod defeats umask so the asserted mode is deterministic.
		if err := os.Chmod(path, perm); err != nil {
			t.Fatal(err)
		}
		return path
	}

	file0600 := writeFile("cred600", []byte("tok123\n"), 0o600)
	file0644 := writeFile("cred644", []byte("tok123"), 0o644)
	emptyFile := writeFile("empty", []byte{}, 0o600)
	tenByte := writeFile("ten", []byte(strings.Repeat("x", 10)), 0o600)
	threeByte := writeFile("three", []byte("abc"), 0o600)
	oneByte := writeFile("one", []byte("z"), 0o600)
	canaryFile := writeFile("canaryFile", []byte("CANARY_PATH_SECRET"), 0o600)
	canaryName := filepath.Join(dir, "CANARY_NAME_SECRET")

	dirPath := filepath.Join(dir, "subdir")
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}

	const notFound = "credential_file: The credential file was not found."
	const invalidSize = "credential_file: The credential file has an invalid size."

	tests := []struct {
		name     string
		path     string
		maxBytes int64
		wantData []byte
		wantMode os.FileMode
		wantErr  string // "" means no error
		noLeak   string // if set, error text must not contain it
	}{
		{"regular 0600", file0600, MaxCredentialBytes, []byte("tok123\n"), 0o600, "", ""},
		{"regular 0644", file0644, MaxCredentialBytes, []byte("tok123"), 0o644, "", ""},
		{"missing", filepath.Join(dir, "nope"), MaxCredentialBytes, nil, 0, notFound, ""},
		{"directory", dirPath, MaxCredentialBytes, nil, 0, notFound, ""},
		{"empty file", emptyFile, MaxCredentialBytes, nil, 0, invalidSize, ""},
		{"oversized", tenByte, 5, nil, 0, invalidSize, ""},
		{"boundary inclusive", threeByte, 3, []byte("abc"), 0o600, "", ""},
		{"content not leaked", canaryFile, 1, nil, 0, invalidSize, "CANARY_PATH_SECRET"},
		{"path not leaked", canaryName, MaxCredentialBytes, nil, 0, notFound, "CANARY_NAME_SECRET"},
		{"maxBytes zero", oneByte, 0, nil, 0, invalidSize, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, mode, err := ReadBoundedFile(tt.path, tt.maxBytes)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ReadBoundedFile(%q, %d) error = %v, want nil", tt.path, tt.maxBytes, err)
				}
				if string(data) != string(tt.wantData) {
					t.Fatalf("ReadBoundedFile(%q, %d) data = %q, want %q", tt.path, tt.maxBytes, data, tt.wantData)
				}
				if mode != tt.wantMode {
					t.Fatalf("ReadBoundedFile(%q, %d) mode = %v, want %v", tt.path, tt.maxBytes, mode, tt.wantMode)
				}
				return
			}
			if err == nil {
				t.Fatalf("ReadBoundedFile(%q, %d) = nil, want error %q", tt.path, tt.maxBytes, tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Fatalf("ReadBoundedFile(%q, %d) error = %q, want %q", tt.path, tt.maxBytes, got, tt.wantErr)
			}
			if tt.noLeak != "" && strings.Contains(err.Error(), tt.noLeak) {
				t.Fatalf("error %q leaks %q", err.Error(), tt.noLeak)
			}
		})
	}
}
