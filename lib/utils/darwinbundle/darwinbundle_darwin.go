// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package darwinbundle

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=11.0
// #cgo LDFLAGS: -framework Foundation
// #include <stdlib.h>
// #include "darwinbundle_darwin.h"
import "C"

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// Path returns a path to the bundle that the current executable comes from.
// If the current executable is a symlink, it resolves the symlink. This is to address a scenario
// where tsh is installed from tsh.pkg and symlinked to /usr/local/bin/tsh, in which case the
// mainBundle function from NSBundle incorrectly points to /usr/local/bin as the bundle path.
// https://developer.apple.com/documentation/foundation/nsbundle/1410786-mainbundle
//
// Returns an error if the dir of the current executable doesn't end with "/Contents/MacOS", likely
// because the executable is not in an app bundle.
func Path() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", trace.Wrap(err)
	}

	absExe, err := utils.NormalizePath(exe, true /* evaluateSymlinks */)
	if err != nil {
		return "", trace.Wrap(err)
	}

	dir := filepath.Dir(absExe)

	const appBundleSuffix = "/Contents/MacOS"
	if !strings.HasSuffix(dir, appBundleSuffix) {
		exeName := filepath.Base(exe)
		return "", trace.NotFound("%s is not in an app bundle", exeName)
	}

	return strings.TrimSuffix(dir, appBundleSuffix), nil
}

// Identifier returns the identifier of the bundle that the current executable comes from. Returns
// either a non-empty string or an error.
func Identifier() (string, error) {
	path, err := Path()
	if err != nil {
		return "", trace.Wrap(err)
	}

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	cIdentifier := C.BundleIdentifier(cPath)
	defer C.free(unsafe.Pointer(cIdentifier))

	identifier := C.GoString(cIdentifier)

	if identifier == "" {
		return "", trace.Errorf("could not get details for bundle under %s", path)
	}

	return identifier, nil
}
