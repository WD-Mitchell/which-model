package config

import "fmt"

type UsageEnabled string

const (
	UsageAuto  UsageEnabled = "auto" // enabled iff ≥1 provider enabled
	UsageTrue  UsageEnabled = "true"
	UsageFalse UsageEnabled = "false"
)

type UsageBackend string

const (
	UsageBackendOff      UsageBackend = "off"
	UsageBackendNative   UsageBackend = "native"
	UsageBackendCodexBar UsageBackend = "codexbar"
)

func ParseUsageBackend(s string) (UsageBackend, error) {
	switch s {
	case string(UsageBackendOff):
		return UsageBackendOff, nil
	case string(UsageBackendNative):
		return UsageBackendNative, nil
	case string(UsageBackendCodexBar):
		return UsageBackendCodexBar, nil
	default:
		return "", fmt.Errorf(`config: usage.backend must be one of "off", "native", "codexbar"; got %s`, s)
	}
}

func (b *UsageBackend) UnmarshalTOML(v interface{}) error {
	value, ok := v.(string)
	if !ok {
		return fmt.Errorf(`config: usage.backend must be a string: "off", "native", or "codexbar"`)
	}
	parsed, err := ParseUsageBackend(value)
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}

func ParseUsageEnabled(s string) (UsageEnabled, error) {
	switch s {
	case "auto":
		return UsageAuto, nil
	case "true":
		return UsageTrue, nil
	case "false":
		return UsageFalse, nil
	default:
		return "", fmt.Errorf(`config: usage.enabled must be one of "auto", "true", "false"; got %s`, s)
	}
}

func (u *UsageEnabled) UnmarshalTOML(v interface{}) error {
	switch value := v.(type) {
	case bool:
		if value {
			*u = UsageTrue
		} else {
			*u = UsageFalse
		}
		return nil
	case string:
		parsed, err := ParseUsageEnabled(value)
		if err != nil {
			return err
		}
		*u = parsed
		return nil
	default:
		return fmt.Errorf(`config: usage.enabled must be a boolean or the string "auto"`)
	}
}
