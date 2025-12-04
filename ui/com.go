package ui

import (
	"syscall"
)

var (
	ole32              = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx = ole32.NewProc("CoInitializeEx")
	procCoUninitialize = ole32.NewProc("CoUninitialize")
)

const (
	COINIT_APARTMENTTHREADED = 0x2
)

// InitializeCOM initializes COM for the current thread
func InitializeCOM() error {
	ret, _, _ := procCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
	// S_OK = 0, S_FALSE = 1 (already initialized), error codes are negative
	if ret != 0 && ret != 1 {
		return syscall.Errno(ret)
	}
	return nil
}

// UninitializeCOM uninitializes COM for the current thread
func UninitializeCOM() {
	procCoUninitialize.Call()
}

