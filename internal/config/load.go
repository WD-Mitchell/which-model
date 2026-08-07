package config

import (
	"os"
	"runtime"
)

type LoadOptions struct {
	Path   string
	Getenv func(string) string
	CWD    string
	Home   string
	GOOS   string
}

func LoadFile(path string) (*Config, error) {
	cfg := Default()
	if err := cfg.DecodeFile(path); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Load(opts LoadOptions) (*Config, error) {
	getenv := opts.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	cwd := opts.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	home := opts.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	goos := opts.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}

	explicit := opts.Path
	if explicit == "" {
		explicit = getenv("WHICH_MODEL_CONFIG")
	}
	if explicit != "" {
		cfg, err := LoadFile(explicit)
		if err != nil {
			return nil, err
		}
		if err := ApplyEnv(cfg, getenv, nil); err != nil {
			return nil, err
		}
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	cfg := Default()
	userFile := UserConfigFile(goos, home, getenv)
	if info, err := os.Stat(userFile); err == nil && !info.IsDir() {
		if err := cfg.DecodeFile(userFile); err != nil {
			return nil, err
		}
	}
	if projectFile, found := ProjectConfigFile(cwd, home); found {
		if err := cfg.DecodeFile(projectFile); err != nil {
			return nil, err
		}
	}
	if err := ApplyEnv(cfg, getenv, nil); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
