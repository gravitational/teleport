package utils

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

func ReadPath(path string) ([]byte, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path %v, err %v", s, err)
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return nil, fmt.Errorf("failed to eval symlinks in path %v, err %v", path, err)
	}
	bytes, err := ioutil.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
