//go:build !nousage

package whichmodel

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/WD-Mitchell/which-model/internal/config"
	"github.com/WD-Mitchell/which-model/internal/output"
	"github.com/WD-Mitchell/which-model/internal/securestore"
	"github.com/WD-Mitchell/which-model/internal/usage/credential"
	"github.com/spf13/cobra"
)

func newAuthMigrateCmd() *cobra.Command {
	var remove, replace bool
	cmd := &cobra.Command{Use: "migrate <provider>", Short: "Explicitly migrate an owned legacy credential to the OS store", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if Global.NoUsage {
			return &UsageError{Message: "credential migration is unavailable with --no-usage"}
		}
		policy, err := readCompanyPolicy()
		if err != nil {
			return err
		}
		opts := credential.MigrationOptions{RemoveSource: remove, Replace: replace}
		if !policy.Managed {
			opts.Transition = enableNativeStorePreference
		}
		var report credential.MigrationReport
		if args[0] == securestore.CatalogAccount {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return homeErr
			}
			paths := config.ResolvePaths(runtime.GOOS, home, os.Getenv)
			report, err = credential.MigrateCatalog(context.Background(), paths.ConfigDir, opts)
		} else {
			store, loadErr := managedCredentialStore()
			if loadErr != nil {
				return loadErr
			}
			// The migration request explicitly selects the new native adapter even
			// when the current personal preference still selects legacy behavior.
			store.Keychain = credential.KeychainFor(true)
			report, err = store.Migrate(context.Background(), args[0], opts)
		}
		if Global.JSON {
			result := struct {
				credential.MigrationReport
				Error string `json:"error,omitempty"`
			}{MigrationReport: report}
			if err != nil {
				result.Error = err.Error()
			}
			if renderErr := output.RenderJSON(cmd.OutOrStdout(), output.OutputEnvelope{}, result); renderErr != nil {
				return renderErr
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "secure store: %s; legacy copy: %s\n", report.SecureStore, report.LegacyCopy)
			if report.RecoveryFile != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "retained recovery file beside the legacy source: %s\n", report.RecoveryFile)
			}
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
			}
		}
		if err != nil {
			return &ReportedError{Err: err}
		}
		return nil
	}}
	cmd.Flags().BoolVar(&remove, "remove-source", false, "remove the selected legacy copy only after secure verification")
	cmd.Flags().BoolVar(&replace, "replace", false, "replace a different existing which-model-owned OS credential")
	return cmd
}

func enableNativeStorePreference() error {
	path := Global.ConfigPath
	if path == "" {
		path = os.Getenv("WHICH_MODEL_CONFIG")
	}
	if path == "" {
		var err error
		path, err = targetConfigPath()
		if err != nil {
			return err
		}
	}
	cfg := config.Default()
	if _, err := os.Stat(path); err == nil {
		cfg, err = config.LoadFile(path)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	auth, err := cfg.LoadAuth()
	if err != nil {
		return err
	}
	auth.UseKeychain = true
	auth.NativeKeychain = true
	if err := cfg.SetAuth(auth); err != nil {
		return err
	}
	data, err := cfg.MarshalTOML()
	if err != nil {
		return err
	}
	if err := config.AtomicWriteFile(path, data); err != nil {
		return err
	}
	effective, err := config.Load(config.LoadOptions{Path: Global.ConfigPath})
	if err != nil {
		return err
	}
	auth, err = effective.LoadAuth()
	if err != nil {
		return err
	}
	if !auth.UseKeychain || !auth.NativeKeychain {
		return &UsageError{Message: "native-store preference is overridden by project or environment settings"}
	}
	return nil
}
