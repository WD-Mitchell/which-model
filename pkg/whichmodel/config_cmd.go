package whichmodel

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/output"
)

func init() {
	register(NewConfigCmd)
	RegisterSchema("config show", map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			"schema_version": map[string]any{"const": "2.0"},
			"_sources":       map[string]any{"type": "object"},
		},
	})
}

// loadConfig resolves the merged config honouring --config.
func loadConfig() (*config.Config, error) {
	return config.Load(config.LoadOptions{Path: Global.ConfigPath})
}

// NewConfigCmd builds the `config` command tree: show|set|path|validate
// (SPEC §11). Registered at order position 11.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "inspect and update which-model configuration",
	}
	cmd.AddCommand(newConfigShowCmd(), newConfigSetCmd(), newConfigPathCmd(), newConfigValidateCmd())
	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "print the fully resolved configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tomlBytes, err := cfg.MarshalTOML()
			if err != nil {
				return err
			}
			if !Global.JSON {
				_, err := Stdout.Write(tomlBytes)
				return err
			}
			var sections map[string]any
			if err := toml.Unmarshal(tomlBytes, &sections); err != nil {
				return err
			}
			home, _ := os.UserHomeDir()
			paths := config.ResolvePaths(runtime.GOOS, home, os.Getenv)
			sources := map[string]any{
				"user_config_file": paths.UserConfigFile,
				"config_dir":       paths.ConfigDir,
				"cache_dir":        paths.CacheDir,
				"state_dir":        paths.StateDir,
			}
			if Global.ConfigPath != "" {
				sources["explicit_config"] = Global.ConfigPath
			}
			sections["_sources"] = sources
			return output.RenderJSON(Stdout, output.OutputEnvelope{}, sections)
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "write a dotted TOML key into the user config file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			if err := validateSetKey(key); err != nil {
				return err
			}
			path, err := targetConfigPath()
			if err != nil {
				return err
			}
			doc, err := readConfigDoc(path)
			if err != nil {
				return err
			}
			if isExistingArray(doc, key) {
				return &UsageError{Message: fmt.Sprintf("cannot set %q: existing value is an array", key)}
			}
			setNestedKey(doc, key, parseTOMLValue(value))
			out, err := toml.Marshal(doc)
			if err != nil {
				return err
			}
			if err := config.AtomicWriteFile(path, out); err != nil {
				return err
			}
			fmt.Fprintf(Stdout, "wrote %s\n", path)
			return nil
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "print the resolved user config file path",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := targetConfigPath()
			if err != nil {
				return err
			}
			fmt.Fprintln(Stdout, path)
			return nil
		},
	}
}

func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "validate the configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				fmt.Fprintln(Stderr, err)
				// annex-d §2.7 (D11): validate errors exit 1, so return a
				// plain error (ConfigError's ExitCode()==2 is stripped).
				return errors.New(err.Error())
			}
			if _, err := loadOutputConfig(cfg); err != nil {
				fmt.Fprintln(Stderr, err)
				return errors.New(err.Error())
			}
			fmt.Fprintln(Stdout, "config is valid")
			return nil
		},
	}
}

// targetConfigPath returns the --config path when set, else the resolved
// user config file path.
func targetConfigPath() (string, error) {
	if Global.ConfigPath != "" {
		return Global.ConfigPath, nil
	}
	home, _ := os.UserHomeDir()
	return config.ResolvePaths(runtime.GOOS, home, os.Getenv).UserConfigFile, nil
}

// validateSetKey rejects empty/blank keys and empty dot segments.
func validateSetKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return &UsageError{Message: "config key must not be empty"}
	}
	for _, seg := range strings.Split(key, ".") {
		if seg == "" {
			return &UsageError{Message: fmt.Sprintf("config key %q has an empty segment", key)}
		}
	}
	return nil
}

// readConfigDoc decodes the target file into a map; a missing file yields an
// empty document.
func readConfigDoc(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

// isExistingArray reports whether the value at the dotted key is a []any.
func isExistingArray(doc map[string]any, key string) bool {
	segments := strings.Split(key, ".")
	node := any(doc)
	for i, seg := range segments {
		m, ok := node.(map[string]any)
		if !ok {
			return false
		}
		next, present := m[seg]
		if !present {
			return false
		}
		if i == len(segments)-1 {
			_, isArr := next.([]any)
			return isArr
		}
		node = next
	}
	return false
}

// setNestedKey writes value at the dotted key, creating intermediate maps.
func setNestedKey(doc map[string]any, key string, value any) {
	segments := strings.Split(key, ".")
	node := doc
	for i, seg := range segments {
		if i == len(segments)-1 {
			node[seg] = value
			return
		}
		child, ok := node[seg].(map[string]any)
		if !ok {
			child = map[string]any{}
			node[seg] = child
		}
		node = child
	}
}

// parseTOMLValue interprets value as a TOML literal: int64 → float64 → bool →
// string (D14).
func parseTOMLValue(v string) any {
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return v
}
