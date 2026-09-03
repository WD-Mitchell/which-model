//go:build !nousage

// Package cursor integrates Cursor Agent's browser-based CLI authentication.
package cursor

import (
	"bufio"
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const fallbackLoginURL = "https://cursor.com/dashboard"

var httpsURLPattern = regexp.MustCompile(`https://[^\s\x1b]+`)

// BrowserLogin is one running `cursor-agent login` process. Cursor Agent owns
// credential persistence; which-model only opens the URL and waits for the CLI
// to report that authentication completed.
type BrowserLogin struct {
	VerificationURL string
	done            <-chan error
}

// StartBrowserLogin starts Cursor Agent without letting it open a second
// browser window, then returns the authorization URL printed by the CLI.
func StartBrowserLogin(ctx context.Context) (*BrowserLogin, error) {
	binary, err := findBinary()
	if err != nil {
		return nil, errors.New("Cursor Agent is required for OAuth sign-in; install cursor-agent first")
	}
	cmd := exec.CommandContext(ctx, binary, "login")
	cmd.Env = append(os.Environ(), "NO_OPEN_BROWSER=1")
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.New("could not start Cursor sign-in")
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, errors.New("could not start Cursor sign-in")
	}

	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 4096), 256*1024)
		for scanner.Scan() {
			if candidate := cursorLoginURL(scanner.Text()); candidate != "" {
				select {
				case urls <- candidate:
				default:
				}
			}
		}
		scanErr := scanner.Err()
		waitErr := cmd.Wait()
		if scanErr != nil {
			done <- errors.New("could not read Cursor sign-in response")
			return
		}
		done <- waitErr
	}()

	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case verificationURL := <-urls:
		return &BrowserLogin{VerificationURL: verificationURL, done: done}, nil
	case waitErr := <-done:
		if waitErr == nil {
			completed := make(chan error, 1)
			completed <- nil
			return &BrowserLogin{VerificationURL: fallbackLoginURL, done: completed}, nil
		}
		return nil, errors.New("Cursor Agent could not begin sign-in")
	case <-timer.C:
		_ = cmd.Process.Kill()
		return nil, errors.New("Cursor Agent did not provide a sign-in URL")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Wait blocks until Cursor Agent has persisted the authenticated session.
func (l *BrowserLogin) Wait(ctx context.Context) error {
	if l == nil || l.done == nil {
		return errors.New("Cursor sign-in was not started")
	}
	select {
	case err := <-l.done:
		if err != nil {
			return errors.New("Cursor sign-in did not complete")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func findBinary() (string, error) {
	for _, name := range []string{"cursor-agent", "agent"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}

func cursorLoginURL(output string) string {
	for _, match := range httpsURLPattern.FindAllString(output, -1) {
		candidate := strings.TrimRight(match, `.,;:!?)\"]}'`)
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme != "https" {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if host == "cursor.com" || strings.HasSuffix(host, ".cursor.com") ||
			host == "cursor.sh" || strings.HasSuffix(host, ".cursor.sh") {
			return parsed.String()
		}
	}
	return ""
}
