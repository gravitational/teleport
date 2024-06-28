// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

//go:build darwin
// +build darwin

package daemon

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=10.15
// #cgo LDFLAGS: -framework Foundation -framework ServiceManagement
// #include "client_darwin.h"
import "C"

import (
	"context"
	"errors"
	"os/exec"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "vnet:daemon")

// Taken from manual for codesign.
const codesignExitCodeVerificationFailed = 1

func IsSigned(ctx context.Context) (bool, error) {
	var result C.BundlePathResult
	defer func() {
		C.free(unsafe.Pointer(result.bundlePath))
	}()
	C.BundlePath(&result)

	bundlePath := C.GoString(result.bundlePath)
	if bundlePath == "" {
		return false, nil
	}

	if err := exec.CommandContext(ctx, "codesign", "--verify", bundlePath).Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			log.DebugContext(ctx, "codesign --verify returned with non-zero exit code",
				"exit_code", exitError.ExitCode(), "bundle_path", bundlePath)

			if exitError.ExitCode() == codesignExitCodeVerificationFailed {
				return false, nil
			}

			// Returning a custom error instead of wrapping err because err is just a cryptic "exit status 2".
			return false, trace.Errorf("failed to check the signature of tsh, codesign returned with exit code %d", exitError.ExitCode())
		}

		return false, trace.Wrap(err, "failed to check the signature of tsh")
	}

	return true, nil
}
