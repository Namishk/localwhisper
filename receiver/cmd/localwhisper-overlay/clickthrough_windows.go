//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

// EnableClickThrough makes the overlay window transparent to mouse input,
// absent from the taskbar, and unable to steal focus. Ebiten owns window
// creation, so we patch the extended style after the HWND appears.
func EnableClickThrough(title string) {
	go func() {
		user32 := syscall.NewLazyDLL("user32.dll")
		findWindow := user32.NewProc("FindWindowW")
		getStyle := user32.NewProc("GetWindowLongPtrW")
		setStyle := user32.NewProc("SetWindowLongPtrW")

		titlePtr, err := syscall.UTF16PtrFromString(title)
		if err != nil {
			return
		}
		const (
			gwlExstyle      = ^uintptr(19) // GWL_EXSTYLE (-20)
			wsExTransparent = 0x00000020
			wsExLayered     = 0x00080000
			wsExToolWindow  = 0x00000080
			wsExNoActivate  = 0x08000000
		)
		for range 100 {
			hwnd, _, _ := findWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
			if hwnd != 0 {
				style, _, _ := getStyle.Call(hwnd, gwlExstyle)
				style |= wsExTransparent | wsExLayered | wsExToolWindow | wsExNoActivate
				setStyle.Call(hwnd, gwlExstyle, style)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}()
}
