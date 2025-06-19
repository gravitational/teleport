// +build plan9 nacl windows

package vt10x

import (
	"os"
)

func ioctl(f *os.File, cmd, p uintptr) error {
	return nil
}

func ResizePty(*os.File) error {
	return nil
}
