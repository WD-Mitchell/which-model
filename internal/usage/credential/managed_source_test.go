//go:build !nousage

package credential

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/usage"
)

func TestNativeManagedCredentialSource(t *testing.T) {
	for _, name := range []string{"keychain", "disabled", "unavailable", "locked", "denied", "legacy"} {
		t.Run(name, func(t *testing.T) {
			keychain := &fakeManagedKeychain{}
			store := ManagedStore{StateDir: t.TempDir(), Keychain: keychain, UseKeychain: name != "disabled" && name != "legacy", NativeKeychain: name != "legacy"}
			wantSource := "managed_file"
			switch name {
			case "keychain":
				wantSource = "keychain"
			case "legacy":
				wantSource = ""
			case "unavailable", "locked", "denied":
				keychain.getErr = &securestore.Error{Kind: securestore.Kind(name)}
				keychain.setErr = keychain.getErr
			}
			before := usage.Credential{Token: "SYNTHETIC_ACCESS", Source: usage.AuthOAuthDeviceFlow, Extra: map[string]string{"account_id": "synthetic-account", "expires_at": "2035-01-01T00:00:00Z", "managed_store": "forged"}}
			if err := store.SaveCredential("codex", before); err != nil {
				t.Fatal(err)
			}
			got, _, err := store.Resolve(context.Background(), "codex")
			if err != nil || !sameCredential(before, got) || got.Extra["managed_store"] != wantSource {
				t.Fatalf("source=%q, want %q; metadata preserved=%t, err=%v", got.Extra["managed_store"], wantSource, sameCredential(before, got), err)
			}
			if name != "keychain" {
				data, err := os.ReadFile(store.Path("codex"))
				if err != nil {
					t.Fatal(err)
				}
				var record managedCredentialFile
				if json.Unmarshal(data, &record) != nil || record.Extra["managed_store"] != "" {
					t.Fatal("source marker was persisted")
				}
				// Old file contents cannot claim that plaintext came from Keychain.
				record.Extra["managed_store"] = "keychain"
				data, _ = json.Marshal(record)
				if err := os.WriteFile(store.Path("codex"), data, 0600); err != nil {
					t.Fatal(err)
				}
				got, _, err = store.Resolve(context.Background(), "codex")
				if err != nil || got.Extra["managed_store"] != wantSource {
					t.Fatalf("stored source marker overrode resolution: source=%q err=%v", got.Extra["managed_store"], err)
				}
			}
		})
	}
}
