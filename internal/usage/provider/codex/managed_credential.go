//go:build !nousage

package codex

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/WD-Mitchell/which-model/internal/security"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

// tokenMetadata reads routing/expiry metadata, not proof of identity or company
// membership. The official provider validates the bearer token on every request.
// Claim names follow openai/codex's login token_data.rs and persist_tokens_async.
func tokenMetadata(token string) (string, *time.Time) {
	if len(token) > 64*1024 {
		return "", nil
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", nil
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", nil
	}
	var claims struct {
		Auth struct {
			AccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
		Expires int64 `json:"exp"`
	}
	if json.Unmarshal(data, &claims) != nil {
		return "", nil
	}
	account := claims.Auth.AccountID
	if !validateIdentifier(account) {
		account = ""
	}
	if claims.Expires <= 0 || claims.Expires > 253402300799 {
		return account, nil
	}
	expires := time.Unix(claims.Expires, 0).UTC()
	return account, &expires
}

func (t Tokens) ManagedCredential() (usage.Credential, error) {
	if security.ValidateOpaqueToken(t.AccessToken) != nil {
		return usage.Credential{}, usage.NewFailureError("unsafe_credential", "The Codex access token is missing or unsafe.")
	}
	account, _ := tokenMetadata(t.IDToken)
	accessAccount, expires := tokenMetadata(t.AccessToken)
	if account == "" {
		account = accessAccount
	}
	if account == "" {
		return usage.Credential{}, usage.NewFailureError("unsafe_credential", "The Codex login did not include required account metadata; sign in again.")
	}
	extra := map[string]string{"account_id": account}
	if expires != nil {
		extra["expires_at"] = expires.Format(time.RFC3339)
	}
	return usage.Credential{Token: t.AccessToken, Extra: extra, Source: usage.AuthOAuthDeviceFlow}, nil
}

func managedFetchCredential(cred usage.Credential) (Credential, error) {
	account := cred.Extra["account_id"]
	if account == "" {
		account, _ = tokenMetadata(cred.Token)
	}
	if security.ValidateOpaqueToken(cred.Token) != nil || !validateIdentifier(account) {
		return Credential{}, &Error{Code: "unsafe_credential", Message: "The securely stored Codex credential needs account metadata; sign in again."}
	}
	if raw := cred.Extra["expires_at"]; raw != "" {
		expires, err := time.Parse(time.RFC3339, raw)
		if err != nil || !expires.After(time.Now()) {
			return Credential{}, &Error{Code: "expired_credential", Message: "The securely stored Codex access token is expired; sign in again."}
		}
	}
	return Credential{Token: cred.Token, AccountID: account}, nil
}
