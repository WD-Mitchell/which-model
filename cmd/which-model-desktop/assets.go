// Emdedded tray template icon rasters (S02 CONTRACTS §4). The 1x and 2x
// black-on-transparent PNGs are embedded so macOS AppKit auto-detects the
// template image set; the 1x is served on all platforms (monochrome
// acceptable at alpha-polish scope, D00 §2.9) and the 2x is retained for the
// retina menu bar. The SVG source of truth lives at assets/tray-icon.svg.
package main

import (
	"embed"
)

// trayIconTemplate is the 18x18 menu-bar glyph (black strokes, transparent
// background), used for the macOS template icon and the Windows/Linux icon.
//
//go:embed assets/tray-iconTemplate.png
var trayIconTemplate []byte

// trayIconTemplate2x is the 36x36 retina raster of the same artwork.
//
//go:embed assets/tray-iconTemplate@2x.png
var trayIconTemplate2x []byte

// trayIconFS exposes both rasters for callers that want the full set.
//
//go:embed assets/tray-iconTemplate.png assets/tray-iconTemplate@2x.png
var trayIconFS embed.FS
