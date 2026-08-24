// Embedded tray template icon rasters (S02 CONTRACTS §4). Black-on-transparent
// PNGs of the mockup's menu-bar glyph, generated from assets/tray-icon.svg by
// assets/gen_tray_icon.go (`go run assets/gen_tray_icon.go`) — run that after
// editing the SVG, never hand-edit the PNGs.
//
// The canvas is 22pt, not the usual 18pt template size, because Wails resizes
// whatever bytes it is given to the status bar thickness:
//
//	systemtray_darwin.m:160  [image setSize:NSMakeSize(thickness, thickness)];
//
// with [[NSStatusBar systemStatusBar] thickness] == 22.0. An 18px raster was
// therefore being scaled 22/18 and then again for retina; a 44px raster maps
// exactly 1:1 onto a 2x menu bar. tray.go hands the 2x raster to
// SetTemplateIcon on darwin and the 1x to SetIcon elsewhere.
package main

import (
	"embed"
)

// trayIconTemplate is the 22x22 (1x) menu-bar glyph, also the Windows/Linux
// tray icon.
//
//go:embed assets/tray-iconTemplate.png
var trayIconTemplate []byte

// trayIconTemplate2x is the 44x44 retina raster — the one macOS renders.
//
//go:embed assets/tray-iconTemplate@2x.png
var trayIconTemplate2x []byte

// trayIconTemplate3x is the 66x66 raster, kept for 3x displays and for
// packaging steps that want the full set.
//
//go:embed assets/tray-iconTemplate@3x.png
var trayIconTemplate3x []byte

// trayIconFS exposes the whole raster set for callers that want to pick a
// scale at runtime.
//
//go:embed assets/tray-iconTemplate.png assets/tray-iconTemplate@2x.png assets/tray-iconTemplate@3x.png
var trayIconFS embed.FS
