//go:build !nousage

package credential

import (
	"context"
	"os"
	"strings"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// EnvResolver resolves Var from the environment; Extra names are copied
// into Credential.Extra when set. Missing / empty-after-trim / unsafe →
// ErrNotFound. Matching surrounding quotes are stripped (SPEC §2).
type EnvResolver struct {
	Var   string
	Extra []string
}

// Resolve reads r.Var. A value that is empty after trimming, or that fails
// security.ValidateOpaqueToken (after stripping one matching surrounding
// quote pair), is "candidate unavailable" → ErrNotFound (SPEC §2, D5). The
// trim only gates emptiness; the token itself is validated as-is so a
// value with embedded whitespace/control characters is rejected, never
// silently cleaned.
func (r *EnvResolver) Resolve(ctx context.Context) (usage.Credential, error) {
	raw := os.Getenv(r.Var)
	if strings.TrimSpace(raw) == "" {
		return Credential{}, ErrNotFound
	}
	token := raw
	if len(token) >= 2 {
		first, last := token[0], token[len(token)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			token = token[1 : len(token)-1]
		}
	}
	if err := security.ValidateOpaqueToken(token); err != nil {
		return Credential{}, ErrNotFound
	}
	var extra map[string]string
	for _, name := range r.Extra {
		if v := os.Getenv(name); v != "" {
			if extra == nil {
				extra = make(map[string]string, len(r.Extra))
			}
			extra[name] = v
		}
	}
	return Credential{Token: token, Extra: extra, Source: usage.AuthEnvVar}, nil
}
