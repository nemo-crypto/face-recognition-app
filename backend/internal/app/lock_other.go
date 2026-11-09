//go:build !linux && !darwin && !freebsd

package app

import "errors"

func Lock(string) (func(), error) {
	return nil, errors.New("当前支持 Linux/macOS/FreeBSD；Windows 请在 WSL 中运行")
}
