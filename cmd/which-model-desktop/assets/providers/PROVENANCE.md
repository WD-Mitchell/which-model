# Provider marks

One monochrome SVG per provider, shown as the menu-bar icon for whoever owns the
current top pick (see `../../providericon.go`, `../../tray.go`).

## Where they came from

The shapes were taken from the `ProviderIcon-*.svg` resources that ship inside
CodexBar.app — the app whose menu bar this behaviour is modelled on. They are
brand marks belonging to their respective owners, used here to identify the
provider a route belongs to. If that provenance is not acceptable for a public
release, the replacements to reach for are [simple-icons](https://simpleicons.org)
(the icon files are CC0; the marks remain their owners' trademarks) or
first-party brand kits.

## What was changed

Only the root `viewBox`, inset by 14% of its larger side on every edge, plus
removal of the root `width`/`height`. The path data is untouched.

The inset is the icon's margin inside the menu bar: Wails resizes whatever image
it is handed to `[[NSStatusBar systemStatusBar] thickness]` (22pt), so art that
fills its viewBox would fill the whole 22pt slot and read as oversized next to
every other status item.

## Adding one

Drop `<name>.svg` in this directory — `//go:embed assets/providers/*.svg` picks
it up. `<name>` is the provider id with everything but letters and digits
removed; if the id and the mark's name differ, add the pair to
`providerIconAliases` instead. The fill colour does not matter: the status item
renders it as a template image, which uses only the alpha channel.
