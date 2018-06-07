// +build !windows

package events

import (
  "archive/tar"
  "io"
  "os"
  "path/filepath"
  "syscall"

  "github.com/gravitational/trace"
)

func openFileForTar(filename string) (*tar.Header, io.ReadCloser, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return nil, nil, trace.ConvertSystemError(err)
	}
	header := tar.Header{
		Name:    filepath.Base(filename),
		Size:    fi.Size(),
		Mode:    int64(fi.Mode()),
		ModTime: fi.ModTime(),
	}
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if ok {
		header.Uid = int(sys.Uid)
		header.Gid = int(sys.Gid)
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, trace.ConvertSystemError(err)
	}
	return &header, f, nil
}
