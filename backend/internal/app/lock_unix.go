//go:build linux || darwin || freebsd

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func Lock(dir string) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("数据目录已被占用或无法加锁: %w", err)
	}
	return func() { _ = f.Close() }, nil
}
