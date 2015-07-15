package utils

import (
	"archive/tar"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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

func WriteArchive(root_directory string, w io.Writer) error {
	ar := tar.NewWriter(w)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsDir() {
			return nil
		}
		// Because of scoping we can reference the external root_directory variable
		new_path := path[len(root_directory):]
		if len(new_path) == 0 {
			return nil
		}
		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		if h, err := tar.FileInfoHeader(info, new_path); err != nil {
			return err
		} else {
			h.Name = new_path
			if err = ar.WriteHeader(h); err != nil {
				return err
			}
		}
		if length, err := io.Copy(ar, fr); err != nil {
			return err
		} else {
			fmt.Println(length)
		}
		return nil
	}

	return filepath.Walk(root_directory, walkFn)
}
