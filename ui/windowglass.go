package ui

import (
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
)

var (
	glassUser32     = syscall.NewLazyDLL("user32.dll")
	glassDwmapi     = syscall.NewLazyDLL("dwmapi.dll")
	procFindWindowW = glassUser32.NewProc("FindWindowW")
	procGetWindowLongPtrW = glassUser32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = glassUser32.NewProc("SetWindowLongPtrW")
	procDwmExtendFrameIntoClientArea = glassDwmapi.NewProc("DwmExtendFrameIntoClientArea")
	procDwmSetWindowAttribute = glassDwmapi.NewProc("DwmSetWindowAttribute")
	procGetForegroundWindow = glassUser32.NewProc("GetForegroundWindow")
)

const (
	GWL_EXSTYLE = uintptr(0xFFFFFFEC) // -20 as uintptr
	WS_EX_LAYERED = 0x80000
	WS_EX_TRANSPARENT = 0x20
	LWA_ALPHA = 0x2
	LWA_COLORKEY = 0x1
)

type DWM_MARGINS struct {
	CxLeftWidth    int32
	CxRightWidth   int32
	CyTopHeight    int32
	CyBottomHeight int32
}

const (
	DWMWA_USE_IMMERSIVE_DARK_MODE = 20
	DWMWA_MICA_EFFECT = 1029
	DWMWA_SYSTEMBACKDROP_TYPE = 38
	DWMWA_BORDER_COLOR = 34
)

// ApplyGlassEffect applies Windows 11 Acrylic/Mica blur effect to a Fyne window
func ApplyGlassEffect(window fyne.Window) {
	// Apply glass effect after a short delay to ensure window is fully created
	go func() {
		// Wait for window to be fully created and shown
		time.Sleep(100 * time.Millisecond)
		
		// Try multiple times to find the window handle
		var hwnd syscall.Handle
		for i := 0; i < 10; i++ {
			hwnd = getFyneWindowHandle(window)
			if hwnd != 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		
		if hwnd == 0 {
			return // Couldn't find window handle
		}

		// Enable window transparency
		style, _, _ := procGetWindowLongPtrW.Call(uintptr(hwnd), GWL_EXSTYLE)
		procSetWindowLongPtrW.Call(uintptr(hwnd), GWL_EXSTYLE, style|uintptr(WS_EX_LAYERED))

		// Extend frame into client area for blur effect
		margins := DWM_MARGINS{
			CxLeftWidth:    -1,
			CxRightWidth:   -1,
			CyTopHeight:    -1,
			CyBottomHeight: -1,
		}
		procDwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))

		// Try to enable Mica effect (Windows 11)
		micaValue := uint32(1)
		procDwmSetWindowAttribute.Call(
			uintptr(hwnd),
			DWMWA_MICA_EFFECT,
			uintptr(unsafe.Pointer(&micaValue)),
			unsafe.Sizeof(micaValue),
		)

		// Set system backdrop type (Windows 11) - Acrylic blur
		backdropType := uint32(3) // DWMSBT_MAINWINDOW (Acrylic)
		procDwmSetWindowAttribute.Call(
			uintptr(hwnd),
			DWMWA_SYSTEMBACKDROP_TYPE,
			uintptr(unsafe.Pointer(&backdropType)),
			unsafe.Sizeof(backdropType),
		)
	}()
}

// getFyneWindowHandle gets the native Windows HWND from a Fyne window
func getFyneWindowHandle(window fyne.Window) syscall.Handle {
	// Fyne uses GLFW, which stores the window handle
	// Try multiple methods to find the window
	
	// Method 1: Find by GLFW class name and title
	title := window.Title()
	if title != "" {
		titlePtr, _ := syscall.UTF16PtrFromString(title)
		
		// Try GLFW30 class (GLFW 3.0+)
		className, _ := syscall.UTF16PtrFromString("GLFW30")
		hwnd, _, _ := procFindWindowW.Call(
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(titlePtr)),
		)
		
		if hwnd != 0 {
			return syscall.Handle(hwnd)
		}
		
		// Try without class name (just by title)
		hwnd, _, _ = procFindWindowW.Call(
			0,
			uintptr(unsafe.Pointer(titlePtr)),
		)
		
		if hwnd != 0 {
			return syscall.Handle(hwnd)
		}
	}
	
	// Method 2: Try to get foreground window (if this is the active window)
	// This is a fallback - less reliable
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd != 0 {
		return syscall.Handle(hwnd)
	}

	return 0
}

