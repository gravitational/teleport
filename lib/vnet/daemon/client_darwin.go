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

//go:build vnetdaemon
// +build vnetdaemon

package daemon

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=10.15
// #cgo LDFLAGS: -framework Foundation -framework ServiceManagement
// #include <stdlib.h>
// #include "client_darwin.h"
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "vnet:daemon")

// RegisterAndCall attempts to register the daemon as a login item, waits for the user to enable it
// and then starts it by sending a message through XPC.
func RegisterAndCall(ctx context.Context, config Config) error {
	bundlePath, err := bundlePath()
	if err != nil {
		return trace.Wrap(err)
	}
	log.DebugContext(ctx, "Registering (if needed) and calling daemon", "bundle_path", bundlePath)
	initialStatus := daemonStatus(bundlePath)

	// If the status is equal to "requires approval" before RegisterAndCall called register, it means
	// that it's not the first time the user tries to start the daemon. In that case, macOS is not
	// going to show the notification about a new login item. Instead, we just open the login items
	// ourselves to direct the user towards enabling the login item.
	if initialStatus == ServiceStatusRequiresApproval {
		C.OpenSystemSettingsLoginItems()
	}

	if initialStatus != ServiceStatusEnabled {
		status, err := register(ctx, bundlePath)
		if err != nil {
			return trace.Wrap(err)
		}

		// Once registered for the first time, the status is likely going to be serviceStatusRequiresApproval.
		if status != ServiceStatusEnabled {
			fmt.Println("To start VNet, please enable the background item for tsh.app in the Login Items section of System Settings.\nWaiting for the background item to be enabled…")
			if err := waitForEnablement(ctx, bundlePath); err != nil {
				return trace.Wrap(err, "waiting for the login item to get enabled")
			}
		}
	}

	if err := startByCalling(ctx, bundlePath, config); err != nil {
		return trace.Wrap(err)
	}

	// TODO(ravicious): Implement monitoring the state of the daemon.
	// Meanwhile, simply block until ctx is canceled.
	<-ctx.Done()
	return trace.Wrap(ctx.Err())
}

func register(ctx context.Context, bundlePath string) (ServiceStatus, error) {
	cBundlePath := C.CString(bundlePath)
	defer C.free(unsafe.Pointer(cBundlePath))

	var result C.RegisterDaemonResult
	defer func() {
		C.free(unsafe.Pointer(result.error_description))
	}()

	C.RegisterDaemon(cBundlePath, &result)

	if !result.ok {
		status := ServiceStatus(result.service_status)
		// Docs for [registerAndReturnError][1] don't seem to cover a situation in which the user adds
		// the launch daemon for the first time. In that case, that method returns "Operation not
		// permitted" error (Code=1, Domain=SMAppServiceErrorDomain). That error doesn't seem to exist
		// in [Service Management Errors][2]. However, all this means is that the launch daemon was
		// added to the login items and the user must now enable the corresponding login item.
		//
		// We can confirm this by looking at the returned service status.
		//
		// [1]: https://developer.apple.com/documentation/servicemanagement/smappservice/register()?language=objc
		// [2]: https://developer.apple.com/documentation/servicemanagement/service-management-errors?language=objc
		if status == ServiceStatusRequiresApproval {
			log.DebugContext(ctx, "Daemon successfully added, but it requires approval",
				"ignored_error", C.GoString(result.error_description))
			return status, nil
		}

		log.DebugContext(ctx, "Registering the daemon has failed", "service_status", status)
		return -1, trace.Errorf("registering daemon failed, %s", C.GoString(result.error_description))
	}

	return ServiceStatus(result.service_status), nil
}

const waitingForEnablementTimeout = time.Minute

var waitingForEnablementTimeoutExceeded = errors.New("the login item was not enabled within the timeout")

// waitForEnablement periodically checks if the status of the daemon has changed to
// [serviceStatusEnabled]. This happens when the user approves the login item in system settings.
func waitForEnablement(ctx context.Context, bundlePath string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeoutCause(ctx, waitingForEnablementTimeout, waitingForEnablementTimeoutExceeded)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-ticker.C:
			switch status := daemonStatus(bundlePath); status {
			case ServiceStatusEnabled:
				return nil
			case ServiceStatusRequiresApproval:
				// Continue waiting for the user to approve the login item.
			case ServiceStatusNotRegistered, ServiceStatusNotFound:
				// Something happened to the service since we started waiting, abort.
				return trace.Errorf("encountered unexpected service status %q", status)
			default:
				return trace.Errorf("encountered unknown service status %q", status)
			}
		}
	}
}

// DaemonStatus returns the status of the background item responsible for launching the daemon.
func DaemonStatus() (ServiceStatus, error) {
	bundlePath, err := bundlePath()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return daemonStatus(bundlePath), nil
}

func daemonStatus(bundlePath string) ServiceStatus {
	cBundlePath := C.CString(bundlePath)
	defer C.free(unsafe.Pointer(cBundlePath))

	return ServiceStatus(C.DaemonStatus(cBundlePath))
}

// https://developer.apple.com/documentation/servicemanagement/smappservice/status-swift.enum?language=objc
type ServiceStatus int

const (
	ServiceStatusNotRegistered    ServiceStatus = 0
	ServiceStatusEnabled          ServiceStatus = 1
	ServiceStatusRequiresApproval ServiceStatus = 2
	ServiceStatusNotFound         ServiceStatus = 3
)

func (s ServiceStatus) String() string {
	switch s {
	case ServiceStatusNotRegistered:
		return "not registered"
	case ServiceStatusEnabled:
		return "enabled"
	case ServiceStatusRequiresApproval:
		return "requires approval"
	case ServiceStatusNotFound:
		return "not found"
	default:
		return strconv.Itoa(int(s))
	}
}

// bundlePath returns a path to the bundle that the current executable comes from.
// If the current executable is a symlink, it resolves the symlink. This is to address a scenario
// where tsh is installed from tsh.pkg and symlinked to /usr/local/bin/tsh, in which case the
// mainBundle function from NSBundle incorrectly points to /usr/local/bin as the bundle path.
// https://developer.apple.com/documentation/foundation/nsbundle/1410786-mainbundle
//
// Returns an error if the dir of the current executable doesn't end with "/Contents/MacOS", likely
// because the executable is not in an app bundle.
func bundlePath() (string, error) {
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
		log.DebugContext(context.Background(), "Current executable is likely outside of app bundle", "exe", absExe)
		return "", trace.NotFound("%s is not in an app bundle", exeName)
	}

	return strings.TrimSuffix(dir, appBundleSuffix), nil
}

func startByCalling(ctx context.Context, bundlePath string, config Config) error {
	// C.StartVnet might hang if the daemon cannot be successfully spawned.
	const daemonStartTimeout = 20 * time.Second
	ctx, cancel := context.WithTimeoutCause(ctx, daemonStartTimeout,
		errors.New("could not connect to the VNet daemon within the timeout"))
	defer cancel()

	defer C.InvalidateDaemonClient()

	errC := make(chan error, 1)

	go func() {
		var pinner runtime.Pinner
		defer pinner.Unpin()

		req := C.StartVnetRequest{
			bundle_path: C.CString(bundlePath),
			vnet_config: &C.VnetConfig{
				socket_path: C.CString(config.SocketPath),
				ipv6_prefix: C.CString(config.IPv6Prefix),
				dns_addr:    C.CString(config.DNSAddr),
				home_path:   C.CString(config.HomePath),
			},
		}
		defer func() {
			C.free(unsafe.Pointer(req.bundle_path))
			C.free(unsafe.Pointer(req.vnet_config.socket_path))
			C.free(unsafe.Pointer(req.vnet_config.ipv6_prefix))
			C.free(unsafe.Pointer(req.vnet_config.dns_addr))
			C.free(unsafe.Pointer(req.vnet_config.home_path))
		}()
		// Structs passed directly as arguments to cgo functions are automatically pinned.
		// However, structs within structs have to be pinned by hand.
		pinner.Pin(req.vnet_config)

		var res C.StartVnetResult
		defer func() {
			C.free(unsafe.Pointer(res.error_domain))
			C.free(unsafe.Pointer(res.error_description))
		}()

		// This call gets unblocked when C.InvalidateDaemonClient is called when startByCalling exits.
		C.StartVnet(&req, &res)

		if !res.ok {
			errC <- trace.Errorf("could not start VNet daemon: %v (%v)", C.GoString(res.error_description), C.GoString(res.error_domain))
			return
		}

		errC <- nil
	}()

	select {
	case <-ctx.Done():
		return trace.Wrap(context.Cause(ctx))
	case err := <-errC:
		return trace.Wrap(err, "connecting to the VNet daemon")
	}
}
