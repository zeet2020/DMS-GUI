package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	ensureWebKitSandbox()
	app := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "DMS",
		Description: "UPnP/DLNA media server",
		Icon:        icon,
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ProgramName: "DMS",
			// Keep running in the system tray when the window is closed.
			DisableQuitOnLastWindowClosed: true,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "DMS — UPnP/DLNA Media Server",
		Width:            820,
		Height:           600,
		MinWidth:         680,
		MinHeight:        360,
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})

	// Closing the window hides it to the system tray instead of quitting, so the
	// dms server keeps serving in the background. Quit is only via the tray menu.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})

	setupSystemTray(wailsApp, window, app)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// setupSystemTray adds a tray icon with a menu to show the window, start/stop the
// server and quit. This is the feature that motivated the move to Wails v3 — the
// app can now run minimized to the tray while serving.
func setupSystemTray(wailsApp *application.App, window *application.WebviewWindow, app *App) {
	tray := wailsApp.SystemTray.New()
	tray.SetLabel("DMS")
	tray.SetIcon(icon)

	menu := wailsApp.NewMenu()
	menu.Add("Show Window").OnClick(func(*application.Context) {
		window.Show()
		window.Focus()
	})
	menu.AddSeparator()
	menu.Add("Start Server").OnClick(func(*application.Context) {
		if err := app.StartServer(app.LoadConfig()); err != nil {
			_, _ = app.logw.Write([]byte(">>> tray start: " + err.Error() + "\n"))
		}
	})
	menu.Add("Stop Server").OnClick(func(*application.Context) {
		_ = app.StopServer()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		_ = app.StopServer()
		wailsApp.Quit()
	})
	tray.SetMenu(menu)

	// Left-clicking the tray icon brings the window back (where supported).
	tray.OnClick(func() {
		window.Show()
		window.Focus()
	})
}
