package ui

import (
	"syscall"
	"unsafe"
)

var (
	toastUser32          = syscall.NewLazyDLL("user32.dll")
	toastKernel32        = syscall.NewLazyDLL("kernel32.dll")
	procSetTimer         = toastUser32.NewProc("SetTimer")
	procKillTimer        = toastUser32.NewProc("KillTimer")
	procDestroyWindow    = toastUser32.NewProc("DestroyWindow")
	procCreateWindowExW  = toastUser32.NewProc("CreateWindowExW")
	procDefWindowProcW   = toastUser32.NewProc("DefWindowProcW")
	procRegisterClassExW = toastUser32.NewProc("RegisterClassExW")
	procGetSystemMetrics = toastUser32.NewProc("GetSystemMetrics")
	procSetWindowTextW   = toastUser32.NewProc("SetWindowTextW")
	procSetWindowPos     = toastUser32.NewProc("SetWindowPos")
	procShowWindowWin    = toastUser32.NewProc("ShowWindow")
	procUpdateWindow     = toastUser32.NewProc("UpdateWindow")
	procGetMessageW      = toastUser32.NewProc("GetMessageW")
	procTranslateMessage = toastUser32.NewProc("TranslateMessage")
	procDispatchMessageW = toastUser32.NewProc("DispatchMessageW")
	procPostQuitMessage  = toastUser32.NewProc("PostQuitMessage")
	procGetModuleHandleW = toastKernel32.NewProc("GetModuleHandleW")
)

const (
	WS_POPUP          = 0x80000000
	WS_VISIBLE        = 0x10000000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_NOACTIVATE  = 0x08000000
	
	WM_TIMER          = 0x0113
	WM_PAINT          = 0x000F
	WM_DESTROY        = 0x0002
	
	HWND_TOPMOST      = ^uintptr(0) // -1
	SWP_NOMOVE        = 0x0002
	SWP_NOSIZE        = 0x0001
	SWP_SHOWWINDOW    = 0x0040
	
	SM_CXSCREEN       = 0
	SM_CYSCREEN       = 1
	
	ID_TIMER_CLOSE    = 1
)

type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type MSG struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type POINT struct {
	X, Y int32
}

var (
	toastHwnd syscall.Handle
	toastText string
)

func toastWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TIMER:
		if wParam == ID_TIMER_CLOSE {
			procKillTimer.Call(uintptr(hwnd), ID_TIMER_CLOSE)
			procDestroyWindow.Call(uintptr(hwnd))
			procPostQuitMessage.Call(0)
			return 0
		}
	case WM_PAINT:
		// Simple text rendering - Windows will handle it
		return 0
	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// ShowToast displays a non-invasive toast notification
func ShowToast(title, message string) {
	// Note: Console visibility is managed by main.go based on debug flag
	// Don't hide console here as it may interfere with debug mode
	
	// Get module handle
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	
	// Register window class for toast
	className, _ := syscall.UTF16PtrFromString("LoLKindBotToast")
	wc := WNDCLASSEX{
		Size:      uint32(unsafe.Sizeof(WNDCLASSEX{})),
		WndProc:   syscall.NewCallback(toastWndProc),
		Instance:  syscall.Handle(hInstance),
		ClassName: className,
	}
	
	ret, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		// Class might already be registered, continue anyway
	}
	
	// Get screen dimensions (reuse from wininput.go)
	cxScreen, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	cyScreen, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	
	// Toast size
	width := int32(350)
	height := int32(100)
	
	// Position in bottom-right corner with margin
	x := int32(cxScreen) - width - 20
	y := int32(cyScreen) - height - 60
	
	// Create window
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	hwnd, _, _ := procCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(titlePtr)),
		WS_POPUP|WS_VISIBLE,
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		0,
		0,
		hInstance,
		0,
	)
	
	if hwnd == 0 {
		return
	}
	
	toastHwnd = syscall.Handle(hwnd)
	
	// Set window text (message) - reuse from wininput.go
	msgPtr, _ := syscall.UTF16PtrFromString(message)
	procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(msgPtr)))
	
	// Set window position (ensure topmost)
	procSetWindowPos.Call(
		uintptr(hwnd),
		HWND_TOPMOST,
		0,
		0,
		0,
		0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW,
	)
	
	// Show window - reuse from wininput.go
	procShowWindowWin.Call(uintptr(hwnd), 1) // SW_SHOWNORMAL
	procUpdateWindow.Call(uintptr(hwnd))
	
	// Set timer to auto-close after 3 seconds
	procSetTimer.Call(uintptr(hwnd), ID_TIMER_CLOSE, 3000, 0)
	
	// Run message loop in goroutine (non-blocking)
	go func() {
		var msg MSG
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if ret == 0 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
}

