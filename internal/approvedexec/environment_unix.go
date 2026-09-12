//go:build darwin || linux

package approvedexec

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

func Environment() ([]string, error) {
	uid := strconv.Itoa(os.Getuid())
	account, err := lookupAccount(uid)
	if err != nil || account == nil || account.Uid != uid || account.Username == "" || !filepath.IsAbs(account.HomeDir) {
		return nil, refusal("OS user environment is unavailable")
	}
	home := account.HomeDir
	locale := "C.UTF-8"
	if runtime.GOOS == "darwin" {
		locale = "en_US.UTF-8"
	}
	env := []string{"HOME=" + home, "USER=" + account.Username, "LOGNAME=" + account.Username, "PATH=/usr/bin:/bin", "LANG=" + locale, "TMPDIR=/tmp"}
	if runtime.GOOS == "linux" {
		run := "/run/user/" + uid
		env = append(env, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_CACHE_HOME="+filepath.Join(home, ".cache"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "XDG_STATE_HOME="+filepath.Join(home, ".local", "state"), "XDG_RUNTIME_DIR="+run, "DBUS_SESSION_BUS_ADDRESS=unix:path="+filepath.Join(run, "bus"))
	}
	return env, nil
}
