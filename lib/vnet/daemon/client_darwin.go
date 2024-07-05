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

package daemon

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=10.15
// #cgo LDFLAGS: -framework Foundation -framework ServiceManagement
// #include "client_darwin.h"
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
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

// RegisterAndCall attempts to register the daemon as a login item, waits for the user to enable it
// and then starts it by sending a message through XPC.
func RegisterAndCall(ctx context.Context, socketPath, ipv6Prefix, dnsAddr string) error {
	initialStatus := daemonStatus()
	// If the status is equal to "requires approval" before RegisterAndCall called register, it means
	// that it's not the first time the user tries to start the daemon. In that case, macOS is not
	// going to show the notification about a new login item. Instead, we just open the login items
	// ourselves to direct the user towards enabling the login item.
	if initialStatus == serviceStatusRequiresApproval {
		C.OpenSystemSettingsLoginItems()
	}

	status, err := register(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if status != serviceStatusEnabled {
		if err := waitForEnablement(ctx); err != nil {
			return trace.Wrap(err, "waiting for the login item to get enabled")
		}
	}

	return trace.NotImplemented("RegisterAndCall is not fully implemented yet")
}

func register(ctx context.Context) (serviceStatus, error) {
	var result C.RegisterDaemonResult

	C.RegisterDaemon(&result)
	defer func() {
		C.free(unsafe.Pointer(result.error_description))
	}()

	if !result.ok {
		status := serviceStatus(result.service_status)
		// Docs for registerAndReturnError [1] don't seem to cover a situation in which the user adds
		// the launch daemon for the first time. In that case, that method returns "Operation not permitted"
		// error (Code=1, Domain=SMAppServiceErrorDomain). That error doesn't seem to exist in Service
		// Management Errors [2]. However, all this means is that the launch daemon was added to the
		// login items and the user must now enable the corresponding login item.
		//
		// We can confirm this by looking at the returned service status.
		//
		// [1] https://developer.apple.com/documentation/servicemanagement/smappservice/register()?language=objc
		// [2] https://developer.apple.com/documentation/servicemanagement/service-management-errors?language=objc
		if status == serviceStatusRequiresApproval {
			log.DebugContext(ctx, "Daemon successfully added, but it requires approval",
				"ignored_error", C.GoString(result.error_description))
			return status, nil
		}

		log.DebugContext(ctx, "Registering the daemon has failed", "service_status", status)
		return 0, trace.Errorf("registering daemon failed, %s", C.GoString(result.error_description))
	}

	return serviceStatus(result.service_status), nil
}

const waitingForEnablementTimeout = time.Minute

var waitingForEnablementTimeoutExceeded = errors.New("the login item was not enabled within the timeout")

// waitForEnablement periodically checks if the status of the daemon has changed to
// [serviceStatusEnabled]. This happens when the user approves the login item in system settings.
func waitForEnablement(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeoutCause(ctx, waitingForEnablementTimeout, waitingForEnablementTimeoutExceeded)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-ticker.C:
			switch status := daemonStatus(); status {
			case serviceStatusEnabled:
				return nil
			case serviceStatusRequiresApproval:
				// Continue waiting for the user to approve the login item.
			case serviceStatusNotRegistered, serviceStatusNotFound:
				// Something happened to the service since we started waiting, abort.
				return trace.Errorf("encountered unexpected service status %q", status)
			default:
				return trace.Errorf("encountered unknown service status %q", status)
			}
		}
	}
}

func daemonStatus() serviceStatus {
	return serviceStatus(C.DaemonStatus())
}

type serviceStatus int

// https://developer.apple.com/documentation/servicemanagement/smappservice/status-swift.enum?language=objc
const (
	serviceStatusNotRegistered    serviceStatus = 0
	serviceStatusEnabled          serviceStatus = 1
	serviceStatusRequiresApproval serviceStatus = 2
	serviceStatusNotFound         serviceStatus = 3
)

func (s serviceStatus) String() string {
	switch s {
	case serviceStatusNotRegistered:
		return "not registered"
	case serviceStatusEnabled:
		return "enabled"
	case serviceStatusRequiresApproval:
		return "requires approval"
	case serviceStatusNotFound:
		return "not found"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}
