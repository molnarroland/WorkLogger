// Command worklogger is the WorkLogger desktop app: it collects git commits, meetings, chats and work
// sessions into draft worklogs and sends them to Jira or Tempo.
package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
	_ "time/tzdata"

	"github.com/molnarroland/WorkLogger/app"
	"github.com/molnarroland/WorkLogger/internal/core"
	"github.com/molnarroland/WorkLogger/internal/git"
	"github.com/molnarroland/WorkLogger/internal/platform"
	"github.com/molnarroland/WorkLogger/internal/store"
	"github.com/molnarroland/WorkLogger/internal/webview"

	// Windows has no zoneinfo database

	spartannative "github.com/spartan-native/host-wails"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Angular builds straight into frontend/dist (see outputPath in angular.json).
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	application.RegisterEvent[app.CollectProgress](app.EventProgress)
	application.RegisterEvent[app.CollectDone](app.EventDone)
	application.RegisterEvent[app.ActivityLine](app.EventActivity)
}

const keyringService = "WorkLogger"

func main() {
	config, err := os.UserConfigDir() // %AppData% on Windows, ~/.config on Linux, ~/Library/Application Support on macOS
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Join(config, "WorkLogger")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Fatal(err)
	}
	days := core.Days{Loc: time.Local}
	path := filepath.Join(dir, "worklogger.sqlite")
	// Encrypted at rest with a key made on the first run and kept in the OS keyring.
	key, err := platform.DatabaseKey(keyringService, store.Encrypted(path))
	if err != nil {
		log.Fatalf("Cannot open the database: %v", err)
	}
	db, err := store.OpenEncrypted(path, key, days)
	if err != nil {
		log.Fatalf("Cannot open the database: %v", err)
	}
	defer db.Close()

	var wails *application.App
	var mainWin *application.WebviewWindow
	// Answers of the scripts run in the Google and HRmaster windows come back over this bridge.
	bridge := webview.NewBridge()
	svc := app.New(app.Deps{
		Store: db, Days: days, Clock: platform.SystemClock{},
		Secrets: &platform.Keyring{Service: keyringService},
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		Git:     git.Git{Run: platform.ExecRunner{}},
		Shell: app.Shell{
			// Only the main window hears the app's events: Wails' Event.Emit runs them in every window, and a page
			// in a site window could listen in (docs/ARCHITECTURE.md, Trust boundaries).
			Emit: func(name string, data any) {
				if mainWin != nil {
					mainWin.DispatchWailsEvent(&application.CustomEvent{Name: name, Data: data})
				}
			},
			OpenURL: func(u string) error { return wails.Browser.OpenURL(u) },
			ChooseFolder: func() (string, error) {
				return wails.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).
					CanCreateDirectories(true).PromptForSingleSelection()
			},
			SaveFile: func(name string) (string, error) {
				return wails.Dialog.SaveFile().SetFilename(name).CanCreateDirectories(true).PromptForSingleSelection()
			},
			Reveal: func(path string) error { return wails.Env.OpenFileManager(path, true) },
		},
		// Each HRmaster and Google profile signs in in its own window (app.WebSession); dir keeps their state.
		NewSite: func(sp app.SiteSpec) app.Site {
			return &webview.Session{
				App: func() *application.App { return wails }, Bridge: bridge, Source: sp.ID, Kind: sp.Kind,
				Name: sp.Name, Home: sp.Home, SignInURL: sp.SignInURL, SignedInAt: sp.SignedInAt, StateFile: sp.StateFile,
			}
		},
		StateDir: dir,
	})
	wails = application.New(application.Options{
		Name:        "WorkLogger",
		Description: "Month-end worklogs from git, calendars and chats",
		// spartannative reads the OS accent, light/dark, contrast, transparency, motion and text size for the
		// frontend's wailsHost() and shows native menus and dialogs. Register it once.
		Services: []application.Service{application.NewService(svc), application.NewService(spartannative.New())},
		// Results of the scripts run in the HRmaster and Google Chat windows.
		RawMessageHandler: bridge.RawMessage,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// Site windows and foreign pages can't reach the runtime (bound methods, events).
			Middleware: webview.GuardRuntime(func(m string) { app.ReportRefused(svc, m) }),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	mainWin = wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "WorkLogger",
		Width:     1280,
		Height:    820,
		MinWidth:  960,
		MinHeight: 600,
		URL:       "/",
		// The frontend draws the title bar (<sn-title-bar>): no OS frame on Windows and Linux, where its window
		// buttons act on this window through wailsHost(); macOS keeps the traffic lights over a hidden-inset bar.
		Frameless: runtime.GOOS != "darwin",
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 38,
		},
	})
	// The hidden HRmaster/Google Chat windows never really close, so Windows (which quits only when no window is
	// left) would keep running without the main window. Closing the main window quits the app.
	mainWin.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { wails.Quit() })

	if err := wails.Run(); err != nil {
		log.Fatal(err)
	}
}
