//go:build darwin && !nousage

package securestore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// keychain is used only by native tests to isolate a disposable keychain. Native
// production instances use the OS user's search list; no env/config selects it.
type darwinStore struct{ keychain string }

func Native() Store { return darwinStore{} }

func (s darwinStore) args(args ...string) []string {
	if s.keychain != "" {
		return append(args, s.keychain)
	}
	return args
}

func classifyDarwin(err error) error {
	if err == nil {
		return nil
	}
	var exited *exec.ExitError
	if errors.As(err, &exited) {
		// Security.framework OSStatus is truncated to the process's exit byte.
		switch exited.ExitCode() {
		case 44:
			return &Error{Missing} // errSecItemNotFound (-25300)
		case 36, 29:
			return &Error{Locked} // interaction not allowed/required
		case 51, 128:
			return &Error{Denied} // authentication failed/user canceled
		}
	}
	return &Error{Unavailable}
}

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 128*1024 {
		return 0, errors.New("native output limit")
	}
	return b.Buffer.Write(p)
}

func runSecurity(input string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/security", args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C"}
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var output boundedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if err != nil {
		return nil, classifyDarwin(err)
	}
	return output.Bytes(), nil
}

func (s darwinStore) Get(service, account string) (string, error) {
	data, err := runSecurity("", s.args("find-generic-password", "-s", service, "-wa", account)...)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(data))
	// Compatibility with entries written by the existing pinned go-keyring.
	if encoded, ok := strings.CutPrefix(value, "go-keyring-base64:"); ok {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return "", &Error{Unavailable}
		}
		return string(data), nil
	}
	if encoded, ok := strings.CutPrefix(value, "go-keyring-encoded:"); ok {
		data, err := hex.DecodeString(encoded)
		if err != nil {
			return "", &Error{Unavailable}
		}
		return string(data), nil
	}
	return value, nil
}

// security's parser consumes escapes inside quotes and does not concatenate
// adjacent quoted words like a shell. Escape within one argument instead.
func securityQuote(value string) string {
	return "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(value) + "\""
}

func (s darwinStore) Set(service, account, value string) error {
	encoded := "go-keyring-base64:" + base64.StdEncoding.EncodeToString([]byte(value))
	args := s.args("add-generic-password", "-U", "-s", service, "-a", account, "-w", encoded)
	for i := range args {
		args[i] = securityQuote(args[i])
	}
	command := strings.Join(args, " ") + "\n"
	if len(command) >= 4096 {
		return &Error{TooLarge}
	}
	// The interactive utility may exit successfully after an individual command
	// fails. Never claim persistence without reading the exact value back.
	if _, err := runSecurity(command, "-i"); err != nil {
		return err
	}
	got, err := s.Get(service, account)
	if err != nil {
		return err
	}
	if got != value {
		return &Error{Unavailable}
	}
	return nil
}

func (s darwinStore) Delete(service, account string) error {
	_, err := runSecurity("", s.args("delete-generic-password", "-s", service, "-a", account)...)
	return err
}
