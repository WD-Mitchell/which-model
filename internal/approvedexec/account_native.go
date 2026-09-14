//go:build (darwin || (linux && cgo)) && !osusergo

package approvedexec

import "os/user"

func lookupAccount(uid string) (*user.User, error) {
	// These builds use the native OS account database, including NSS on Linux.
	// os/user's environment-derived fallback is confined to the pure-Go build;
	// account_passwd.go deliberately does not call os/user's cached lookup.
	return user.LookupId(uid)
}
