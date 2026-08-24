//go:build !darwin || ios

// Non-macOS stub. Traffic lights are an AppKit concept; other platforms draw
// their own window controls, which the page does not overlap.
package main

import "github.com/wailsapp/wails/v3/pkg/application"

func positionTrafficLights(_ *application.WebviewWindow) {}
