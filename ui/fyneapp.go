package ui

import (
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

var (
	fyneAppInstance fyne.App
	fyneAppOnce     sync.Once
	appRunning      sync.WaitGroup
	uiQueue         chan func()
	uiQueueOnce     sync.Once
	appReady        chan bool
	appReadyOnce    sync.Once
	appReadyFlag    bool
	appReadyMutex   sync.RWMutex
)

// initUIQueue initializes the UI operation queue
func initUIQueue() {
	uiQueueOnce.Do(func() {
		uiQueue = make(chan func(), 100)
		appReady = make(chan bool, 1)
	})
}

// isAppReady checks if the app is ready (thread-safe)
func isAppReady() bool {
	appReadyMutex.RLock()
	defer appReadyMutex.RUnlock()
	return appReadyFlag
}

// setAppReady marks the app as ready (thread-safe)
func setAppReady() {
	appReadyMutex.Lock()
	appReadyFlag = true
	appReadyMutex.Unlock()

	// Signal on channel (non-blocking)
	select {
	case appReady <- true:
	default:
		// Channel already has a value, that's fine
	}
}

// RunOnMain ensures a function runs on the Fyne app thread
// Fyne v2 requires UI operations to run on the main thread
// We use fyne.Do() to ensure thread safety
func RunOnMain(fn func()) {
	// Ensure app is initialized
	app := getFyneApp()
	if app == nil {
		return
	}

	// Wait for app to be ready (with timeout)
	if isAppReady() {
		// App is ready, use fyne.Do() to ensure thread safety
		fyne.Do(fn)
	} else {
		// Wait a bit for app to become ready
		select {
		case <-appReady:
			// App is ready, use fyne.Do() to ensure thread safety
			fyne.Do(fn)
		case <-time.After(2 * time.Second):
			// App not ready yet, but try anyway using fyne.Do()
			// This ensures thread safety even if app.Run() hasn't fully started
			fyne.Do(fn)
		}
	}
}

// GetFyneApp returns a singleton Fyne app instance (exported for main.go)
func GetFyneApp() fyne.App {
	return getFyneApp()
}

// StartFyneApp initializes the Fyne app and returns the app instance
// app.Run() must be called directly from the main goroutine
func StartFyneApp() fyne.App {
	fyneAppOnce.Do(func() {
		fyneAppInstance = app.NewWithID("com.lolkindbot.app")
		// Use beautiful glass morphism theme
		fyneAppInstance.Settings().SetTheme(NewGlassTheme())

		// Initialize UI queue
		initUIQueue()

		// Signal that app is initialized (but not running yet)
		setAppReady()
	})

	// Return the app instance - main goroutine must call app.Run() directly
	return fyneAppInstance
}

// getFyneApp returns a singleton Fyne app instance
// This just returns the instance - StartFyneApp() must be called first
func getFyneApp() fyne.App {
	// Ensure app is initialized (StartFyneApp should be called from main())
	if fyneAppInstance == nil {
		// Fallback: try to initialize if StartFyneApp wasn't called
		// This shouldn't happen, but provides a safety net
		fyneAppOnce.Do(func() {
			fyneAppInstance = app.NewWithID("com.lolkindbot.app")
			fyneAppInstance.Settings().SetTheme(NewGlassTheme())
			initUIQueue()
			setAppReady()
		})
	}
	return fyneAppInstance
}
