//go:build windows

package approvedexec

import (
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/sys/windows"
)

func Environment() ([]string, error) {
	// CreateEnvironmentBlock with inherit=false derives the loaded OS user's
	// environment without expanding the attacker's current process overrides.
	// KnownFolderPath alone can fail when USERPROFILE/SystemRoot are overridden.
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &token); err != nil {
		return nil, refusal("OS user environment is unavailable")
	}
	defer token.Close()
	block, err := token.Environ(false)
	if err != nil {
		return nil, refusal("OS user environment is unavailable")
	}
	values := map[string]string{}
	for _, entry := range block {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[strings.ToUpper(key)] = value
		}
	}
	home, err := token.GetUserProfileDirectory()
	if err != nil {
		return nil, refusal("OS user profile is unavailable")
	}
	validPath := func(value string) bool {
		return filepath.IsAbs(value) && filepath.Clean(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
	}
	if !validPath(home) {
		return nil, refusal("OS user profile is unavailable")
	}
	env := []string{"LANG=en_US.UTF-8", "USERPROFILE=" + home}
	for _, key := range []string{"APPDATA", "LOCALAPPDATA", "PROGRAMDATA"} {
		value := values[key]
		if !validPath(value) {
			return nil, refusal("OS user environment is unavailable")
		}
		env = append(env, key+"="+value)
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return nil, refusal("OS system environment is unavailable")
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, refusal("OS system environment is unavailable")
	}
	local := values["LOCALAPPDATA"]
	env = append(env, "HOME="+home, "HOMEDRIVE="+filepath.VolumeName(home), "HOMEPATH="+home[len(filepath.VolumeName(home)):], "SystemRoot="+root, "WINDIR="+root, "PATH="+system, "TEMP="+filepath.Join(local, "Temp"), "TMP="+filepath.Join(local, "Temp"))
	return env, nil
}
