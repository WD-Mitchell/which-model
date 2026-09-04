package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/shopspring/decimal"
)

func (c *Config) MarshalTOML() ([]byte, error) {
	doc := make(map[string]any)
	usage := make(map[string]any)
	switch c.Usage.Enabled {
	case UsageAuto:
		usage["enabled"] = "auto"
	case UsageTrue:
		usage["enabled"] = true
	case UsageFalse:
		usage["enabled"] = false
	case "":
		usage["enabled"] = "auto"
	default:
		return nil, &ConfigError{Kind: KindInvalidValue, Key: "usage.enabled", Err: fmt.Errorf("unknown usage state %q", c.Usage.Enabled)}
	}
	switch c.Usage.Backend {
	case UsageBackendOff, "":
		usage["backend"] = string(UsageBackendOff)
	case UsageBackendNative:
		usage["backend"] = string(UsageBackendNative)
	case UsageBackendCodexBar:
		usage["backend"] = string(UsageBackendCodexBar)
	default:
		return nil, &ConfigError{Kind: KindInvalidValue, Key: "usage.backend", Err: fmt.Errorf("unknown usage backend %q", c.Usage.Backend)}
	}

	doc["usage"] = usage

	providers := make(map[string]any)
	ids := make([]string, 0, len(c.Providers))
	for id := range c.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		provider := c.Providers[id]
		values := map[string]any{"enabled": provider.Enabled}
		if provider.Priority != 0 {
			values["priority"] = provider.Priority
		}
		if !provider.Weight.IsZero() && !provider.Weight.Equal(decimal.NewFromInt(1)) {
			values["weight"] = provider.Weight.String()
		}
		if provider.CacheTTL != 0 {
			values["cache_ttl"] = durationString(provider.CacheTTL)
		}
		if len(provider.SourcePreference) > 0 {
			values["source_preference"] = provider.SourcePreference
		}
		if provider.CredentialPath != "" {
			values["credential_path"] = provider.CredentialPath
		}
		if provider.TrustedFallbackOrigin != "" {
			values["trusted_fallback_origin"] = provider.TrustedFallbackOrigin
		}
		if len(provider.Accounts) > 0 {
			// []map[string]any, not []any: renderSection dispatches on that
			// exact type to emit [[providers.<id>.accounts]]. As []any it fell
			// through to the scalar branch and rendered a top-level
			// [[accounts]] table, which would corrupt the file on round-trip.
			accounts := make([]map[string]any, 0, len(provider.Accounts))
			for _, account := range provider.Accounts {
				entry := map[string]any{"name": account.Name, "kind": account.Kind}
				if account.Ref != "" {
					entry["ref"] = account.Ref
				}
				accounts = append(accounts, entry)
			}
			values["accounts"] = accounts
		}
		providers[id] = values
	}
	if len(providers) > 0 {
		doc["providers"] = providers
	}

	for key, value := range c.raw {
		if key == "usage" || key == "providers" || strings.HasPrefix(key, "providers.") {
			continue
		}
		doc[key] = deepCopyRaw(value)
	}

	envKeys := make([]string, 0, len(c.env))
	for key := range c.env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		setKey(doc, key, inferEnvValue(c.env[key]))
	}

	// B01 SPEC §2.6: fixed render list, then any remaining top-level raw
	// tables in ascending name order (unknown sections must not be dropped).
	fixed := []string{"usage", "auth", "scoring", "strategy", "bands", "catalog", "output", "gui", "profiles", "groups", "harnesses", "favourites", "routes", "providers"}
	rendered := make(map[string]bool, len(fixed))
	var builder strings.Builder
	for _, name := range fixed {
		rendered[name] = true
		section, ok := doc[name].(map[string]any)
		if !ok {
			continue
		}
		if err := renderSection(&builder, name, section); err != nil {
			return nil, err
		}
	}
	remainder := make([]string, 0, len(doc))
	for name, value := range doc {
		if rendered[name] {
			continue
		}
		if _, ok := value.(map[string]any); ok {
			remainder = append(remainder, name)
		}
	}
	sort.Strings(remainder)
	for _, name := range remainder {
		if err := renderSection(&builder, name, doc[name].(map[string]any)); err != nil {
			return nil, err
		}
	}
	return []byte(builder.String()), nil
}

func deepCopyRaw(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copyValue := make(map[string]any, len(typed))
		for key, nested := range typed {
			copyValue[key] = deepCopyRaw(nested)
		}
		return copyValue
	case []string:
		return append([]string(nil), typed...)
	case []any:
		copyValue := make([]any, len(typed))
		for index, nested := range typed {
			copyValue[index] = deepCopyRaw(nested)
		}
		return copyValue
	case []map[string]any:
		copyValue := make([]map[string]any, len(typed))
		for index, nested := range typed {
			copyValue[index] = deepCopyRaw(nested).(map[string]any)
		}
		return copyValue
	default:
		return value
	}
}

func inferEnvValue(value string) any {
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return int64(parsed)
	}
	return value
}

func setKey(doc map[string]any, dotted string, value any) {
	segments := strings.Split(dotted, ".")
	if len(segments) == 0 || segments[0] == "" {
		return
	}
	current := doc
	for _, segment := range segments[:len(segments)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[segment] = next
		}
		current = next
	}
	current[segments[len(segments)-1]] = value
}

func durationString(value time.Duration) string {
	if value >= time.Minute && value%time.Minute == 0 {
		hours := value / time.Hour
		minutes := (value % time.Hour) / time.Minute
		switch {
		case hours > 0 && minutes > 0:
			return fmt.Sprintf("%dh%dm", hours, minutes)
		case hours > 0:
			return fmt.Sprintf("%dh", hours)
		default:
			return fmt.Sprintf("%dm", minutes)
		}
	}
	return value.String()
}

func renderSection(builder *strings.Builder, name string, section map[string]any) error {
	scalars := make(map[string]any)
	subs := make([]string, 0)
	arrays := make([]string, 0)
	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		switch section[key].(type) {
		case map[string]any:
			subs = append(subs, key)
		case []map[string]any:
			arrays = append(arrays, key)
		default:
			scalars[key] = section[key]
		}
	}
	if len(scalars) > 0 {
		data, err := toml.Marshal(scalars)
		if err != nil {
			return &ConfigError{Kind: KindInvalidValue, Key: name, Err: err}
		}
		builder.WriteString("[")
		builder.WriteString(name)
		builder.WriteString("]\n")
		builder.Write(data)
		builder.WriteByte('\n')
	}
	for _, key := range subs {
		nested := section[key].(map[string]any)
		if err := renderSection(builder, name+"."+key, nested); err != nil {
			return err
		}
	}
	for _, key := range arrays {
		for _, nested := range section[key].([]map[string]any) {
			data, err := toml.Marshal(nested)
			if err != nil {
				return &ConfigError{Kind: KindInvalidValue, Key: name + "." + key, Err: err}
			}
			builder.WriteString("[[")
			builder.WriteString(name)
			builder.WriteByte('.')
			builder.WriteString(key)
			builder.WriteString("]]\n")
			builder.Write(data)
			builder.WriteByte('\n')
		}
	}
	return nil
}
