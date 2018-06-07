package events

import (
  "archive/tar"
  "io"
  "os"
  "path/filepath"

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
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, trace.ConvertSystemError(err)
	}
	return &header, f, nil
}
