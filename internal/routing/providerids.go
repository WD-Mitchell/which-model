package routing

// builtinCatalogueSlugs maps the three builtin usage-provider ids onto the
// models.dev provider slugs whose catalogue records they serve (fixture-
// confirmed: "anthropic", "openai", "github-copilot" — see
// internal/catalog/fetch/modelsdev and config/providers.toml, which keys
// excluded_models by these slugs). The ids differ because ours name the
// SUBSCRIPTION ("claude" = a Claude Code plan serving anthropic models), not
// the vendor.
var builtinCatalogueSlugs = map[string]string{
	"claude":  "anthropic",
	"codex":   "openai",
	"copilot": "github-copilot",
}

// CatalogueSlugFor returns the models.dev provider slug whose catalogue
// records provider id serves. Non-builtin ids are their own slug: the
// Providers "Add provider" flow only ever offers models.dev slugs as ids,
// so the identity holds by construction.
func CatalogueSlugFor(id string) string {
	if slug, ok := builtinCatalogueSlugs[id]; ok {
		return slug
	}
	return id
}
