//go:build nousage

package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/WD-Mitchell/which-model/pkg/offlinedesktop"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var frontend embed.FS

func main() {
	assets, err := fs.Sub(frontend, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	app := application.New(application.Options{Name: "which-model Offline", Assets: application.AssetOptions{Handler: application.BundledAssetFileServer(assets)}, Services: []application.Service{application.NewService(&offlinedesktop.API{})}})
	app.Window.NewWithOptions(application.WebviewWindowOptions{Title: "which-model — Offline ranking", Width: 880, Height: 700, MinWidth: 500, MinHeight: 400, URL: "/offline.html", DevToolsEnabled: false, DefaultContextMenuDisabled: true})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
