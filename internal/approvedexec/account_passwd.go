//go:build (linux && !cgo) || ((darwin || linux) && osusergo)

package approvedexec

import (
	"bufio"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
)

func lookupAccount(uid string) (*user.User, error) {
	// Pure-Go os/user.LookupId can return Current's cached HOME/USER fallback.
	// Read its authoritative source directly; an NSS-only or absent account
	// needs a native-lookup build, never an identity inferred from environment.
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return accountFromPasswd(f, uid)
}

func accountFromPasswd(r io.Reader, uid string) (*user.User, error) {
	// Bound both total I/O and individual records. Missing, malformed or
	// oversized data is refused by Environment with a fixed diagnostic.
	scanner := bufio.NewScanner(io.LimitReader(r, 4<<20))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) != 7 || fields[2] != uid || fields[0] == "" || strings.ContainsAny(fields[0][:1], "+-") {
			continue
		}
		if _, err := strconv.ParseUint(fields[2], 10, 32); err != nil {
			continue
		}
		if _, err := strconv.ParseUint(fields[3], 10, 32); err != nil {
			continue
		}
		return &user.User{Uid: fields[2], Gid: fields[3], Username: fields[0], HomeDir: fields[5]}, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, refusal("OS user environment is unavailable")
}
