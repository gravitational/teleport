/*
Copyright 2022 Gravitational, Inc.

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

package botfs

import (
	"io/fs"
	"os"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTBot,
})

type SymlinksMode string

const (
	// SymlinksInsecure does allow resolving symlink paths and does not issue
	// any symlink-related warnings.
	SymlinksInsecure SymlinksMode = "insecure"

	// SymlinksTrySecure attempts to write files securely and avoid symlink
	// attacks, but falls back with a warning if the necessary OS / kernel
	// support is missing.
	SymlinksTrySecure SymlinksMode = "try-secure"

	// SymlinksSecure attempts to write files securely and fails with an error
	// if the operation fails. This should be the default on systems were we
	// expect it to be supported.
	SymlinksSecure SymlinksMode = "secure"
)

// DefaultMode is the preferred permissions mode for bot files.
const DefaultMode fs.FileMode = 0600

// openStandard attempts to open the given path for writing with O_CREATE set.
func openStandard(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, DefaultMode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return file, nil
}

// createStandard creates an empty file or directory at the given path without
// attempting to prevent symlink attacks.
func createStandard(path string, isDir bool) error {
	if isDir {
		if err := os.Mkdir(path, DefaultMode); err != nil {
			return trace.Wrap(err)
		}
	} else {
		f, err := openStandard(path)
		if err != nil {
			return trace.Wrap(err)
		}

		f.Close()
	}

	return nil
}
