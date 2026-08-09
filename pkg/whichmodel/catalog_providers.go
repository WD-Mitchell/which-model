package whichmodel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/WD-Mitchell/which-model/internal/catalog/fetch/modelsdev"
	"github.com/WD-Mitchell/which-model/internal/output"
)

// cacheReader is a test seam over readCache.
var cacheReader = readCache

// providerRow is one provider's catalogue view for `catalog providers`.
type providerRow struct {
	ID       string
	Models   []modelsdev.ProviderModel
	Excluded []string
}

// renderProviders builds one providerRow per configured provider id
// (alphabetical), applying subset filtering after sorting.
func renderProviders(configured map[string][]string, catalogue []modelsdev.ProviderModel, subset []string) ([]providerRow, error) {
	ids := make([]string, 0, len(configured))
	for id := range configured {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if len(subset) > 0 {
		want := make(map[string]bool, len(subset))
		for _, s := range subset {
			want[s] = true
		}
		filtered := ids[:0:0]
		for _, id := range ids {
			if want[id] {
				filtered = append(filtered, id)
			}
		}
		ids = filtered
	}

	rows := make([]providerRow, 0, len(ids))
	for _, id := range ids {
		excluded := make(map[string]bool, len(configured[id]))
		for _, m := range configured[id] {
			excluded[m] = true
		}
		var models []modelsdev.ProviderModel
		for _, cm := range catalogue {
			if cm.Provider != id {
				continue
			}
			if excluded[cm.ModelID] {
				continue
			}
			models = append(models, cm)
		}
		rows = append(rows, providerRow{ID: id, Models: models, Excluded: configured[id]})
	}
	return rows, nil
}

// excludedText renders the trailing excluded-count column.
func excludedText(excluded []string) string {
	if len(excluded) == 0 {
		return "0 excluded"
	}
	return fmt.Sprintf("%d excluded (%s)", len(excluded), strings.Join(excluded, ", "))
}

// providersText writes the fixed-width text view (annex-d §2.5).
func providersText(w interface{ Write([]byte) (int, error) }, rows []providerRow) {
	for _, r := range rows {
		fmt.Fprintf(w, "%-16s %-9s %s\n", r.ID, fmt.Sprintf("%d models", len(r.Models)), excludedText(r.Excluded))
	}
}

// providersJSON builds the {"<id>": [{"id","name","reasoning"}]} payload.
func providersJSON(rows []providerRow) map[string]any {
	doc := make(map[string]any, len(rows))
	for _, r := range rows {
		entries := make([]map[string]any, len(r.Models))
		for i, m := range r.Models {
			entries[i] = map[string]any{
				"id":        m.ModelID,
				"name":      m.Name,
				"reasoning": m.EffortLevels,
			}
		}
		doc[r.ID] = entries
	}
	return doc
}

func newProvidersCmd() *cobra.Command {
	f := &catalogFlags{}
	cmd := &cobra.Command{
		Use: "providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProviders(f, cmd)
		},
	}
	f.Bind(cmd)
	return cmd
}

func runProviders(f *catalogFlags, cmd *cobra.Command) error {
	_, res, err := catalogPreamble(f)
	if err != nil {
		return err
	}

	providerConfigPath := f.ProviderConfig
	if providerConfigPath == "" {
		providerConfigPath = res.ProviderConfigPath
	}
	prov, err := loadProviderConfig(providerConfigPath)
	if err != nil {
		return err
	}
	if err := validateProviders(f.Providers, prov); err != nil {
		return err
	}

	catalogue, ok, err := cacheReader(res.CatalogueCachePath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("provider catalogue not found at %s; run 'which-model catalog benchmarks' (or '--refresh-benchmarks') to collect it", res.CatalogueCachePath)
	}

	rows, err := renderProviders(prov, catalogue, f.Providers)
	if err != nil {
		return err
	}

	if Global.JSON {
		return output.RenderJSON(cmd.OutOrStdout(), output.OutputEnvelope{}, providersJSON(rows))
	}
	providersText(cmd.OutOrStdout(), rows)
	return nil
}
