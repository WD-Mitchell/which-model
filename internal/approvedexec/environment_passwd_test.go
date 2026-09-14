//go:build (linux && !cgo) || ((darwin || linux) && osusergo)

package approvedexec

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
)

// Native CI runs this static test binary in an empty chroot, so even UID 0
// has no passwd entry. No host account database is changed.
func TestApprovedEnvironmentMissingAccount(t *testing.T) {
	uid := strconv.Itoa(os.Getuid())
	data, err := os.ReadFile("/etc/passwd")
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 && fields[2] == uid {
			t.Skip("requires a UID absent from the account database")
		}
	}
	t.Setenv("HOME", "/ENV_HOME_CANARY")
	t.Setenv("USER", "ENV_USER_CANARY")
	// Prime os/user's cache with its environment-derived current-user fallback.
	if _, err := user.Current(); err != nil {
		t.Fatal(err)
	}
	env, err := Environment()
	if err == nil || len(env) != 0 {
		t.Fatalf("missing OS identity must refuse an environment: %q, %v", env, err)
	}
	if strings.Contains(err.Error(), "CANARY") {
		t.Fatal("refusal leaked inherited identity data")
	}
}

func TestApprovedEnvironmentPasswdData(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"account", "other:x:123:456::/home/other:/bin/sh\nmanaged:x:987:654::/home/managed:/bin/sh\n", true},
		{"missing", "other:x:123:456::/home/other:/bin/sh\n", false},
		{"empty", "", false},
		{"malformed", "managed:x:987:invalid::/home/managed:/bin/sh\n", false},
		{"NIS marker", "+managed:x:987:654::/home/managed:/bin/sh\n", false},
		{"oversized record", strings.Repeat("x", 65536) + "\nmanaged:x:987:654::/home/managed:/bin/sh\n", false},
		{"oversized database", strings.Repeat("# comment\n", 500000) + "managed:x:987:654::/home/managed:/bin/sh\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", "/ENV_HOME_CANARY")
			t.Setenv("USER", "ENV_USER_CANARY")
			account, err := accountFromPasswd(strings.NewReader(tc.data), "987")
			if !tc.valid {
				if err == nil || account != nil {
					t.Fatalf("invalid identity accepted: %+v, %v", account, err)
				}
				return
			}
			if err != nil || account.Uid != "987" || account.Username != "managed" || account.HomeDir != "/home/managed" {
				t.Fatalf("database identity was not preserved: %+v, %v", account, err)
			}
		})
	}
}
