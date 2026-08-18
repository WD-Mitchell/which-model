// Package main is the Wails v3 desktop host. S01 stub: one empty window;
// S02 replaces this body with the full bootstrap (S00 SPEC §2.1).
package main

import "github.com/wailsapp/wails/v3/pkg/application"

func main() {
	app := application.New(application.Options{Name: "which-model"})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "which-model",
	})
	if err := app.Run(); err != nil {
		panic(err)
	}
}
