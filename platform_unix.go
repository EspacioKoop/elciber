//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func replaceFile(from, to string) error { return os.Rename(from, to) }
func configureProcess(cmd *exec.Cmd)    { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func terminateProcess(p *os.Process, force bool) error {
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	return syscall.Kill(-p.Pid, sig)
}
func acquireDataLock(dir string) (func(), error) {
	p := filepath.Join(dir, ".lock")
	if st, err := os.Lstat(p); err == nil && !st.Mode().IsRegular() {
		return nil, errDisk
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errDisk
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errLocked
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
