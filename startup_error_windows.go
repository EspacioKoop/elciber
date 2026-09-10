//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func reportStartupError(err error) {
	text, _ := syscall.UTF16PtrFromString(err.Error())
	title, _ := syscall.UTF16PtrFromString("El Ciber")
	box := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	_, _, _ = box.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}
