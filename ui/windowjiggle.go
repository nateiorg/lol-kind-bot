package ui

import (
	"sync"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

var (
	jiggleUser32        = syscall.NewLazyDLL("user32.dll")
	procGetWindowRect   = jiggleUser32.NewProc("GetWindowRect")
	procGetWindowThreadProcessId = jiggleUser32.NewProc("GetWindowThreadProcessId")
)

type RECT struct {
	Left, Top, Right, Bottom int32
}

// addJiggleEffectSimple adds a jiggle effect that animates content when window is moved
// Returns the jiggle container for chaining with other effects
func addJiggleEffectSimple(window fyne.Window, content fyne.CanvasObject) fyne.CanvasObject {
	var (
		jiggling    bool
		jiggleMutex sync.Mutex
	)

	// Wrap content in a container without layout for manual positioning
	jiggleContainer := container.NewWithoutLayout(content)
	originalPos := fyne.NewPos(0, 0)
	content.Move(originalPos)

	// Function to trigger jiggle animation
	jiggle := func() {
		jiggleMutex.Lock()
		if jiggling {
			jiggleMutex.Unlock()
			return
		}
		jiggling = true
		jiggleMutex.Unlock()

		// Animate in a goroutine to avoid blocking
		go func() {
			jiggleDistance := float32(8) // More noticeable jiggle
			duration := 15 * time.Millisecond
			
			// Create a sequence of small movements for jiggle effect
			sequence := []struct {
				dx, dy float32
			}{
				{jiggleDistance, 0},           // Right
				{0, jiggleDistance},           // Down
				{-jiggleDistance, 0},          // Left
				{0, -jiggleDistance},          // Up
				{jiggleDistance / 2, 0},       // Right (smaller)
				{0, jiggleDistance / 2},       // Down (smaller)
				{-jiggleDistance / 2, 0},     // Left (smaller)
				{0, -jiggleDistance / 2},      // Up (smaller)
				{0, 0},                       // Return to center
			}

			for _, move := range sequence {
				newPos := originalPos.Add(fyne.NewPos(move.dx, move.dy))
				content.Move(newPos)
				content.Refresh()
				time.Sleep(duration)
			}

			// Ensure we're back at original position
			content.Move(originalPos)
			content.Refresh()

			jiggleMutex.Lock()
			jiggling = false
			jiggleMutex.Unlock()
		}()
	}

	// Use Windows API to detect window movement and resizing
	go func() {
		// Wait a bit for window to be fully created
		time.Sleep(200 * time.Millisecond)
		
		ticker := time.NewTicker(30 * time.Millisecond) // Check more frequently for smoother jiggle
		defer ticker.Stop()
		
		// Get window handle - try multiple times
		var hwnd syscall.Handle
		for i := 0; i < 20; i++ {
			hwnd = getWindowHandle(window)
			if hwnd != 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		
		if hwnd == 0 {
			return // Can't track without handle
		}
		
		// Track last known position and size
		lastX, lastY := getWindowPosition(hwnd)
		lastWidth, lastHeight := getWindowSize(hwnd)
		initialized := false
		
		for {
			select {
			case <-ticker.C:
				currentX, currentY := getWindowPosition(hwnd)
				currentWidth, currentHeight := getWindowSize(hwnd)
				
				if !initialized {
					// Initialize tracking
					lastX, lastY = currentX, currentY
					lastWidth, lastHeight = currentWidth, currentHeight
					initialized = true
					continue
				}
				
				// Check if position changed
				dx := currentX - lastX
				dy := currentY - lastY
				if (dx > 1 || dx < -1) || (dy > 1 || dy < -1) {
					// Window moved - trigger jiggle
					jiggle()
				}
				
				// Check if size changed (resizing)
				dw := currentWidth - lastWidth
				dh := currentHeight - lastHeight
				if (dw > 2 || dw < -2) || (dh > 2 || dh < -2) {
					// Window resized - trigger jiggle
					jiggle()
				}
				
				lastX, lastY = currentX, currentY
				lastWidth, lastHeight = currentWidth, currentHeight
			case <-time.After(60 * time.Second):
				// Longer timeout for active windows
				return
			}
		}
	}()

	// Return the jiggle container instead of setting it directly
	// This allows chaining with other effects
	return jiggleContainer
}

// getWindowPosition gets the window position using Windows API
func getWindowPosition(hwnd syscall.Handle) (int32, int32) {
	if hwnd == 0 {
		return 0, 0
	}
	
	var rect RECT
	ret, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return 0, 0
	}
	
	return rect.Left, rect.Top
}

// getWindowSize gets the window size using Windows API
func getWindowSize(hwnd syscall.Handle) (int32, int32) {
	if hwnd == 0 {
		return 0, 0
	}
	
	var rect RECT
	ret, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rect)))
	if ret == 0 {
		return 0, 0
	}
	
	return rect.Right - rect.Left, rect.Bottom - rect.Top
}

// getWindowHandle gets the native window handle from Fyne window
func getWindowHandle(window fyne.Window) syscall.Handle {
	// Use the shared function from windowglass.go
	return getFyneWindowHandle(window)
}
