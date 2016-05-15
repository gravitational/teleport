package utils

import (
	"io"
	"os"
)

// IsFile returns true if a given file path points to an existing file
func IsFile(fp string) bool {
	fi, err := os.Stat(fp)
	if err == nil {
		return !fi.IsDir()
	}
	return false
}

// IsDir is a helper function to quickly check if a given path is a valid directory
func IsDir(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err == nil {
		return fi.IsDir()
	}
	return false
}

// ReadAll is similarl to ioutil.ReadAll, except it doesn't use ever-increasing
// internal buffer, instead asking for the exact buffer size.
//
// This is useful when you want to limit the sze of Read/Writes (websockets)
func ReadAll(r io.Reader, bufsize int) (out []byte, err error) {
	buff := make([]byte, bufsize)
	n := 0
	for err == nil {
		n, err = r.Read(buff)
		if n > 0 {
			out = append(out, buff[:n]...)
		}
	}
	if err == io.EOF {
		err = nil
	}
	return out, err
}
