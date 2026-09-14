//go:build !nousage

package claude

import (
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func (t Tokens) ManagedCredential() (usage.Credential, error) {
	if security.ValidateOpaqueToken(t.AccessToken) != nil {
		return usage.Credential{}, usage.NewFailureError("unsafe_credential", "The Claude access token is missing or unsafe.")
	}
	extra := map[string]string{}
	if t.ExpiresIn > 0 {
		if int64(t.ExpiresIn) > int64((30*24*time.Hour)/time.Second) {
			return usage.Credential{}, usage.NewFailureError("unsafe_credential", "The Claude token expiry is invalid.")
		}
		extra["expires_at"] = time.Now().UTC().Add(time.Duration(t.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	return usage.Credential{Token: t.AccessToken, Extra: extra, Source: usage.AuthOAuthDeviceFlow}, nil
}
