package config

import (
	"encoding"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

func mergeRaw(dst, src map[string]any) {
	for key, value := range src {
		if existing, ok := dst[key]; ok {
			existingMap, existingOK := existing.(map[string]any)
			valueMap, valueOK := value.(map[string]any)
			if existingOK && valueOK {
				mergeRaw(existingMap, valueMap)
				continue
			}
		}
		dst[key] = value
	}
}

func rawLookup(root map[string]any, dotted string) any {
	if root == nil {
		return nil
	}
	var current any = root
	for _, segment := range strings.Split(dotted, ".") {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = mapping[segment]
		if !ok {
			return nil
		}
	}
	return current
}

func (c *Config) DecodeFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConfigError{Kind: KindNotFound, Path: path}
		}
		return &ConfigError{Kind: KindUnreadable, Path: path, Err: err}
	}

	var layer map[string]any
	if _, err := toml.Decode(string(data), &layer); err != nil {
		return &ConfigError{Kind: KindInvalidTOML, Path: path, Err: err}
	}
	if c.raw == nil {
		c.raw = make(map[string]any)
	}
	mergeRaw(c.raw, layer)

	if node := rawLookup(layer, "usage"); node != nil {
		text, err := toml.Marshal(node)
		if err != nil {
			return &ConfigError{Kind: KindInvalidTOML, Path: path, Err: err}
		}
		md, err := toml.Decode(string(text), &c.Usage)
		if err != nil {
			return &ConfigError{Kind: KindInvalidValue, Path: path, Key: "usage.enabled", Err: err}
		}
		if undecoded := md.Undecoded(); len(undecoded) > 0 {
			return &ConfigError{Kind: KindInvalidValue, Path: path, Key: "usage." + undecoded[0].String()}
		}
	}

	if node := rawLookup(layer, "providers"); node != nil {
		providers, ok := node.(map[string]any)
		if !ok {
			return &ConfigError{Kind: KindInvalidValue, Path: path, Key: "providers", Err: errors.New("not a table")}
		}
		if c.Providers == nil {
			c.Providers = make(map[string]ProviderConfig)
		}
		for id, providerNode := range providers {
			providerTable, ok := providerNode.(map[string]any)
			if !ok {
				return &ConfigError{Kind: KindInvalidValue, Path: path, Key: "providers." + id, Err: errors.New("not a table")}
			}
			text, err := toml.Marshal(providerTable)
			if err != nil {
				return &ConfigError{Kind: KindInvalidTOML, Path: path, Err: err}
			}
			provider := c.Providers[id]
			md, err := toml.Decode(string(text), &provider)
			if err != nil {
				return &ConfigError{Kind: KindInvalidValue, Path: path, Key: "providers." + id, Err: err}
			}
			if undecoded := md.Undecoded(); len(undecoded) > 0 {
				return &ConfigError{Kind: KindInvalidValue, Path: path, Key: "providers." + id + "." + undecoded[0].String()}
			}
			c.Providers[id] = provider
		}
	}
	return nil
}

var (
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	durationType         = reflect.TypeOf(time.Duration(0))
)

func (c *Config) UnmarshalKey(key string, out any) error {
	node := rawLookup(c.raw, key)
	if node == nil {
		return nil
	}
	table, ok := node.(map[string]any)
	if !ok {
		return &ConfigError{Kind: KindInvalidValue, Key: key, Err: errors.New("not a table")}
	}
	text, err := toml.Marshal(table)
	if err != nil {
		return &ConfigError{Kind: KindInvalidValue, Key: key, Err: err}
	}
	md, err := toml.Decode(string(text), out)
	if err != nil {
		return &ConfigError{Kind: KindInvalidValue, Key: key, Err: err}
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return &ConfigError{Kind: KindInvalidValue, Key: key + "." + undecoded[0].String()}
	}

	if len(c.env) == 0 {
		return nil
	}
	matched := make(map[string]bool)
	value := reflect.ValueOf(out)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return &ConfigError{Kind: KindInvalidValue, Key: key, Err: errors.New("output must be a non-nil pointer")}
	}
	if err := applyEnvOverlay(c.env, value.Elem(), key, matched); err != nil {
		return err
	}
	prefix := key + "."
	for envKey := range c.env {
		if strings.HasPrefix(envKey, prefix) && !matched[envKey] {
			return &ConfigError{Kind: KindInvalidValue, Key: envKey}
		}
	}
	return nil
}

func applyEnvOverlay(env map[string]string, out reflect.Value, prefix string, matched map[string]bool) error {
	if out.Kind() != reflect.Struct || !out.CanSet() {
		return nil
	}
	outType := out.Type()
	for i := 0; i < outType.NumField(); i++ {
		fieldType := outType.Field(i)
		field := out.Field(i)
		if !field.CanSet() {
			continue
		}
		tag := fieldType.Tag.Get("toml")
		if tag == "" || tag == "-" {
			continue
		}
		segment := strings.SplitN(tag, ",", 2)[0]
		path := prefix + "." + segment

		if value, ok := env[path]; ok {
			matched[path] = true
			target, ok := envTarget(field)
			if ok {
				if err := setEnvValue(target, value, path); err != nil {
					return err
				}
			}
		}

		child, ok := envChild(field, env, path)
		if !ok {
			continue
		}
		if err := applyEnvOverlay(env, child, path, matched); err != nil {
			return err
		}
	}
	return nil
}

func envTarget(field reflect.Value) (reflect.Value, bool) {
	target := field
	if target.Kind() == reflect.Ptr {
		elemType := target.Type().Elem()
		if elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Map || elemType.Kind() == reflect.Array {
			return reflect.Value{}, false
		}
		if elemType.Kind() == reflect.Struct && elemType != durationType && !reflect.PointerTo(elemType).Implements(textUnmarshalerType) {
			return reflect.Value{}, false
		}
		if target.IsNil() {
			target.Set(reflect.New(elemType))
		}
		target = target.Elem()
	}
	if target.Kind() == reflect.Slice || target.Kind() == reflect.Map || target.Kind() == reflect.Array {
		return reflect.Value{}, false
	}
	if target.Kind() == reflect.Struct && target.Type() != durationType && !target.Addr().Type().Implements(textUnmarshalerType) {
		return reflect.Value{}, false
	}
	return target, true
}

func envChild(field reflect.Value, env map[string]string, path string) (reflect.Value, bool) {
	child := field
	if child.Kind() == reflect.Ptr {
		elemType := child.Type().Elem()
		if elemType.Kind() != reflect.Struct || elemType == durationType || reflect.PointerTo(elemType).Implements(textUnmarshalerType) {
			return reflect.Value{}, false
		}
		if child.IsNil() {
			prefix := path + "."
			found := false
			for key := range env {
				if strings.HasPrefix(key, prefix) {
					found = true
					break
				}
			}
			if !found {
				return reflect.Value{}, false
			}
			child.Set(reflect.New(elemType))
		}
		child = child.Elem()
	}
	if child.Kind() != reflect.Struct || child.Type() == durationType || !child.CanSet() || child.Addr().Type().Implements(textUnmarshalerType) {
		return reflect.Value{}, false
	}
	return child, true
}

func setEnvValue(target reflect.Value, value, path string) error {
	if target.Type() == durationType {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return &ConfigError{Kind: KindInvalidValue, Key: path, Err: err}
		}
		target.SetInt(int64(duration))
		return nil
	}
	if target.CanAddr() && target.Addr().Type().Implements(textUnmarshalerType) {
		if err := target.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value)); err != nil {
			return &ConfigError{Kind: KindInvalidValue, Key: path, Err: err}
		}
		return nil
	}
	switch target.Kind() {
	case reflect.String:
		target.SetString(value)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return &ConfigError{Kind: KindInvalidValue, Key: path, Err: err}
		}
		target.SetBool(parsed)
	case reflect.Int:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return &ConfigError{Kind: KindInvalidValue, Key: path, Err: err}
		}
		target.SetInt(parsed)
	}
	return nil
}
