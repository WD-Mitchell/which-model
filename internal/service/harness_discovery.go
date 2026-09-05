package service

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"go.yaml.in/yaml/v3"
)

// Discovery reads bounded local configuration documents only. It never expands
// embedded commands, resolves secrets, contacts providers, or writes those files.
const harnessConfigLimit = 2 << 20

func harnessDocument(path string) map[string]any {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > harnessConfigLimit {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > harnessConfigLimit {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, harnessConfigLimit+1))
	if err != nil || len(data) > harnessConfigLimit {
		return nil
	}
	var out map[string]any
	switch filepath.Ext(path) {
	case ".toml":
		_, err = toml.Decode(string(data), &out)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &out)
	default:
		err = json.Unmarshal(stripJSONComments(data), &out)
	}
	if err != nil {
		return nil
	}
	return out
}

// Preserve string contents (including URLs) while accepting JSONC comments and
// trailing commas used by OpenCode/Kilo/editor settings.
func stripJSONComments(data []byte) []byte {
	out := append([]byte(nil), data...)
	quoted, escaped := false, false
	for i := 0; i < len(out); i++ {
		c := out[i]
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c == '/' && i+1 < len(out) && out[i+1] == '/' {
			for i < len(out) && out[i] != '\n' {
				out[i] = ' '
				i++
			}
		}
		if c == '/' && i+1 < len(out) && out[i+1] == '*' {
			out[i] = ' '
			i++
			out[i] = ' '
			for i+1 < len(out) && !(out[i] == '*' && out[i+1] == '/') {
				if out[i] != '\n' {
					out[i] = ' '
				}
				i++
			}
			if i+1 < len(out) {
				out[i] = ' '
				i++
				out[i] = ' '
			}
		}
	}
	quoted, escaped = false, false
	for i, c := range out {
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c == ',' {
			tail := bytes.TrimSpace(out[i+1:])
			if len(tail) > 0 && (tail[0] == '}' || tail[0] == ']') {
				out[i] = ' '
			}
		}
	}
	return out
}

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func textValue(v any) string      { s, _ := v.(string); return strings.TrimSpace(s) }
func providerAlias(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	switch id {
	case "anthropic", "claude-code":
		return "claude"
	case "openai", "chatgpt", "openai-codex":
		return "codex"
	case "github-copilot", "github_copilot":
		return "copilot"
	case "gemini", "google-gemini":
		return "google"
	case "vertex-ai", "vertex_ai", "google-vertex":
		return "google-vertex"
	case "bedrock":
		return "amazon-bedrock"
	}
	return id
}

func discoverHarnessProviders(home, slug string) []string {
	found := map[string]bool{}
	add := func(id string) {
		id = providerAlias(id)
		if id != "" && providerIDRe.MatchString(id) {
			found[id] = true
		}
	}
	doc := func(rel string) map[string]any { return harnessDocument(filepath.Join(home, filepath.FromSlash(rel))) }
	model := func(v any) {
		s := textValue(v)
		if before, _, ok := strings.Cut(s, "/"); ok {
			add(before)
		}
	}
	keys := func(m map[string]any) {
		for id, v := range m {
			if object(v)["disabled"] != true && object(v)["disable"] != true {
				add(id)
			}
		}
	}
	switch slug {
	case "opencode", "kilo":
		keys(doc(".local/share/" + slug + "/auth.json"))
		for _, ext := range []string{"json", "jsonc"} {
			d := doc(".config/" + slug + "/" + slug + "." + ext)
			keys(object(d["provider"]))
			model(d["model"])
			model(d["small_model"])
			if allow, ok := d["enabled_providers"].([]any); ok {
				allowed := map[string]bool{}
				for _, v := range allow {
					allowed[providerAlias(textValue(v))] = true
				}
				for id := range found {
					if !allowed[id] {
						delete(found, id)
					}
				}
			}
			if deny, ok := d["disabled_providers"].([]any); ok {
				for _, v := range deny {
					delete(found, providerAlias(textValue(v)))
				}
			}
		}
	case "cline":
		keys(object(doc(".cline/data/settings/providers.json")["providers"]))
		d := doc(".cline/data/globalState.json")
		for _, key := range []string{"apiProvider", "actModeApiProvider", "planModeApiProvider"} {
			add(textValue(d[key]))
		}
	case "codex":
		d := doc(".codex/config.toml")
		add(textValue(d["model_provider"]))
		keys(object(d["model_providers"]))
		if len(doc(".codex/auth.json")) > 0 {
			add("codex")
		}
	case "claude":
		for _, path := range []string{".claude/settings.json", ".claude/settings.local.json"} {
			d := doc(path)
			env := object(d["env"])
			switch {
			case textValue(env["CLAUDE_CODE_USE_BEDROCK"]) == "1":
				add("amazon-bedrock")
			case textValue(env["CLAUDE_CODE_USE_VERTEX"]) == "1":
				add("google-vertex")
			case textValue(env["ANTHROPIC_API_KEY"]) != "" || textValue(env["ANTHROPIC_AUTH_TOKEN"]) != "":
				add("claude")
			}
		}
		if len(doc(".claude/.credentials.json")) > 0 || object(doc(".claude.json")["oauthAccount"]) != nil {
			add("claude")
		}
	case "copilot":
		d := doc(".copilot/config.json")
		if len(d) > 0 {
			add("copilot")
		}
	case "cursor":
		if len(doc(".cursor/cli-config.json")) > 0 || len(doc(".config/cursor/auth.json")) > 0 {
			add("cursor")
		}
	case "gemini":
		d := doc(".gemini/settings.json")
		auth := object(object(d["security"])["auth"])
		if textValue(auth["selectedType"]) != "" || len(doc(".gemini/oauth_creds.json")) > 0 {
			add("google")
		}
	case "qwen":
		d := doc(".qwen/settings.json")
		keys(object(d["modelProviders"]))
		if len(doc(".qwen/oauth_creds.json")) > 0 {
			add("alibaba")
		}
	case "goose":
		d := doc(".config/goose/config.yaml")
		add(textValue(d["GOOSE_PROVIDER"]))
		add(textValue(object(d["provider"])["name"]))
		add(textValue(d["provider"]))
	case "aider":
		d := doc(".aider.conf.yml")
		model(d["model"])
		model(d["weak-model"])
		model(d["editor-model"])
		if textValue(d["anthropic-api-key"]) != "" {
			add("claude")
		}
		if textValue(d["openai-api-key"]) != "" {
			add("codex")
		}
	case "continue":
		for _, path := range []string{".continue/config.yaml", ".continue/config.json"} {
			d := doc(path)
			if models, ok := d["models"].([]any); ok {
				for _, m := range models {
					add(textValue(object(m)["provider"]))
				}
			}
		}
	case "crush":
		keys(object(doc(".config/crush/crush.json")["providers"]))
	case "droid":
		for _, path := range []string{".factory/settings.json", ".factory/settings.local.json"} {
			d := doc(path)
			if len(d) > 0 {
				add("factory")
			}
			if models, ok := d["customModels"].([]any); ok {
				for _, m := range models {
					add(textValue(object(m)["provider"]))
				}
			}
		}
	case "amp":
		if len(doc(".config/amp/settings.json")) > 0 {
			add("amp")
		}
	case "antigravity":
		if info, err := os.Stat(filepath.Join(home, ".gemini", "antigravity")); err == nil && info.IsDir() {
			add("antigravity")
		}
	case "windsurf":
		if info, err := os.Stat(filepath.Join(home, ".codeium", "windsurf")); err == nil && info.IsDir() {
			add("windsurf")
		}
	}
	out := make([]string, 0, len(found))
	for id := range found {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

var providerIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Cline distinguishes API and OAuth provider adapters (for example openai and
// openai-codex). Keep the configured adapter when the DTO uses a shared alias.
func clineProviderID(home, provider string) string {
	d := harnessDocument(filepath.Join(home, ".cline", "data", "settings", "providers.json"))
	providers := object(d["providers"])
	last := textValue(d["lastUsedProvider"])
	if _, ok := providers[last]; ok && providerIDRe.MatchString(last) && providerAlias(last) == provider {
		return last
	}
	ids := []string{}
	for id := range providers {
		if providerIDRe.MatchString(id) && providerAlias(id) == provider {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}
