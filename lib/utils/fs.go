/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
)

// EnsureLocalPath makes sure the path exists, or, if omitted results in the subpath in
// default gravity config directory, e.g.
//
// EnsureLocalPath("/custom/myconfig", ".gravity", "config") -> /custom/myconfig
// EnsureLocalPath("", ".gravity", "config") -> ${HOME}/.gravity/config
//
// It also makes sure that base dir exists
func EnsureLocalPath(customPath string, defaultLocalDir, defaultLocalPath string) (string, error) {
	if customPath == "" {
		homeDir := getHomeDir()
		if homeDir == "" {
			return "", trace.BadParameter("no path provided and environment variable %v is not not set", teleport.EnvHome)
		}
		customPath = filepath.Join(homeDir, defaultLocalDir, defaultLocalPath)
	}
	baseDir := filepath.Dir(customPath)
	_, err := StatDir(baseDir)
	if err != nil {
		if trace.IsNotFound(err) {
			if err := MkdirAll(baseDir, teleport.PrivateDirMode); err != nil {
				return "", trace.Wrap(err)
			}
		} else {
			return "", trace.Wrap(err)
		}
	}
	return customPath, nil
}

// MkdirAll creates directory and subdirectories
func MkdirAll(targetDirectory string, mode os.FileMode) error {
	err := os.MkdirAll(targetDirectory, mode)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// RemoveDirCloser removes directory and all it's contents
// when Close is called
type RemoveDirCloser struct {
	Path string
}

// Close removes directory and all it's contents
func (r *RemoveDirCloser) Close() error {
	return trace.ConvertSystemError(os.RemoveAll(r.Path))
}

// IsDir is a helper function to quickly check if a given path is a valid directory
func IsDir(path string) bool {
	fi, err := os.Stat(path)
	if err == nil {
		return fi.IsDir()
	}
	return false
}

// IsFile is a convenience helper to check if the given path is a regular file
func IsFile(path string) bool {
	fi, err := os.Stat(path)
	if err == nil {
		return fi.Mode().IsRegular()
	}
	return false
}

// NormalizePath normalises path, evaluating symlinks and converting local
// paths to absolute
func NormalizePath(path string) (string, error) {
	s, err := filepath.Abs(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	abs, err := filepath.EvalSymlinks(s)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return abs, nil
}

// OpenFile opens  file and returns file handle
func OpenFile(path string) (*os.File, error) {
	newPath, err := NormalizePath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fi, err := os.Stat(newPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if fi.IsDir() {
		return nil, trace.BadParameter("%v is not a file", path)
	}
	f, err := os.Open(newPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return f, nil
}

// StatFile stats path, returns error if it exists but a directory.
func StatFile(path string) (os.FileInfo, error) {
	newPath, err := NormalizePath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fi, err := os.Stat(newPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if fi.IsDir() {
		return nil, trace.BadParameter("%v is not a file", path)
	}
	return fi, nil
}

// StatDir stats directory, returns error if file exists, but not a directory
func StatDir(path string) (os.FileInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	if !fi.IsDir() {
		return nil, trace.BadParameter("%v is not a directory", path)
	}
	return fi, nil
}

// getHomeDir returns the home directory based off the OS.
func getHomeDir() string {
	switch runtime.GOOS {
	case constants.LinuxOS:
		return os.Getenv(teleport.EnvHome)
	case constants.DarwinOS:
		return os.Getenv(teleport.EnvHome)
	case constants.WindowsOS:
		return os.Getenv(teleport.EnvUserProfile)
	}
	return ""
}
