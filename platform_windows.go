//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var moveFileEx = kernel32.NewProc("MoveFileExW")

func replaceFile(from, to string) error {
	a, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	b, err := syscall.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	r, _, e := moveFileEx.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 0x1|0x8)
	if r == 0 {
		return e
	}
	return nil
}
func configureProcess(cmd *exec.Cmd)                   { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
func terminateProcess(p *os.Process, force bool) error { return p.Kill() }
func acquireDataLock(dir string) (func(), error) {
	p, err := syscall.UTF16PtrFromString(filepath.Join(dir, ".lock"))
	if err != nil {
		return nil, errDisk
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, errLocked
	}
	return func() { _ = syscall.CloseHandle(h) }, nil
}
