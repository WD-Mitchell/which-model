//go:build !nousage

package fetch

import (
	"strings"

	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/usage"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
)

const companyCodexBarApprovalMessage = "CodexBar installation approval is unavailable; no credential was delegated"

// Provider failures can contain echoed HTTP or delegated process payloads.
// Company diagnostics retain the canonical code, never that free-form text.
// Native-store messages are fixed application strings with useful remediation.
func minimizeCompanyFailures(snapshots []usage.Snapshot) {
	for i := range snapshots {
		failure := snapshots[i].Failure
		if failure == nil {
			continue
		}
		code := failure.Code
		switch code {
		case "unauthorized", "rate_limited", "provider_status", "expired_credential", "unsupported_response", "login_required", "endpoint_refused", "untrusted_origin", "redirect_refused", "response_too_large", "timeout", "network", "response_json", "credential_file", "credential_json", "unsafe_credential", "access_denied", "device_expired", "fallback_unavailable", "usage_disabled", "usage_compiled_out", "keychain_unavailable", "cookie_unavailable", "signing_failed", "rpc_protocol":
		default:
			code = "provider_status"
		}
		message := "usage unavailable (" + code + "); inspect provider status or sign in again"
		if code == "provider_status" && failure.Message == companyCodexBarApprovalMessage {
			message = companyCodexBarApprovalMessage
		}
		if code == "keychain_unavailable" {
			for _, kind := range []securestore.Kind{securestore.Missing, securestore.Locked, securestore.Denied, securestore.Unavailable, securestore.TooLarge} {
				if fixed := (&securestore.Error{Kind: kind}).Error(); failure.Message == fixed {
					message = fixed
					break
				}
			}
		}
		snapshots[i].Failure = &usage.Failure{Code: code, Message: message}
	}
}

// Warnings are message-only diagnostics. Classify the application-generated
// permission/cache notices, but never copy their path or underlying error text.
// Unknown warnings also get a fixed message so new sources fail closed here.
func minimizeCompanyWarnings(warnings []credential.Warning) {
	for i, warning := range warnings {
		message := "usage warning; inspect provider status and credential settings"
		switch {
		case strings.HasPrefix(warning.Message, "credential file ") && strings.Contains(warning.Message, " has broad permissions"):
			message = "credential file has broad permissions; review before continuing"
		case strings.HasPrefix(warning.Message, "failed to cache usage for provider "):
			message = "usage cache write failed; run privacy cleanup for category results"
		case warning.Message == "system keychain unavailable; using managed credential file":
			message = warning.Message
		}
		warnings[i] = credential.Warning{Message: message}
	}
}
