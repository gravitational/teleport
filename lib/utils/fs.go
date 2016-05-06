package utils

import (
	"io"
	"os"
)

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
// We need this for websockets: they can't deal with huge Reads > 32K
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
