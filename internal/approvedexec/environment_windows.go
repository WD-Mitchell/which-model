//go:build windows

package approvedexec

import (
	"golang.org/x/sys/windows"
	"path/filepath"
)

func Environment() ([]string, error) {
	env := []string{"LANG=en_US.UTF-8"}
	var home, local string
	for _, item := range []struct {
		name string
		id   *windows.KNOWNFOLDERID
	}{{"USERPROFILE", windows.FOLDERID_Profile}, {"APPDATA", windows.FOLDERID_RoamingAppData}, {"LOCALAPPDATA", windows.FOLDERID_LocalAppData}, {"PROGRAMDATA", windows.FOLDERID_ProgramData}} {
		path, err := windows.KnownFolderPath(item.id, 0)
		if err != nil || !filepath.IsAbs(path) {
			return nil, refusal("OS user environment is unavailable")
		}
		env = append(env, item.name+"="+path)
		if item.name == "USERPROFILE" {
			home = path
		}
		if item.name == "LOCALAPPDATA" {
			local = path
		}
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return nil, refusal("OS system environment is unavailable")
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, refusal("OS system environment is unavailable")
	}
	env = append(env, "HOME="+home, "HOMEDRIVE="+filepath.VolumeName(home), "HOMEPATH="+home[len(filepath.VolumeName(home)):], "SystemRoot="+root, "WINDIR="+root, "PATH="+system, "TEMP="+filepath.Join(local, "Temp"), "TMP="+filepath.Join(local, "Temp"))
	return env, nil
}
