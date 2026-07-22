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

package daemon

// #include <stdlib.h>
// #include "service_darwin.h"
import "C"

import (
	"context"
	"errors"
	"time"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils/darwinbundle"
)

// Start starts an XPC listener and waits for it to receive a message with VNet config.
// Once the message is received, it executes [workFn] with that config.
func Start(ctx context.Context, workFn func(context.Context, Config) error) error {
	bundlePath, err := darwinbundle.Path()
	if err != nil {
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "Starting daemon", "version", teleport.Version, "bundle_path", bundlePath)

	cBundlePath := C.CString(bundlePath)
	defer C.free(unsafe.Pointer(cBundlePath))

	var result C.DaemonStartResult
	defer func() {
		C.free(unsafe.Pointer(result.error_domain))
		C.free(unsafe.Pointer(result.error_description))
	}()
	C.DaemonStart(cBundlePath, &result)
	if !result.ok {
		errorDomain := C.GoString(result.error_domain)
		errorCode := int(result.error_code)

		if errorDomain == vnetErrorDomain && errorCode == errorCodeMissingCodeSigningIdentifiers {
			return trace.Wrap(errMissingCodeSigningIdentifiers)
		}

		return trace.Errorf("could not start daemon: %s", C.GoString(result.error_description))
	}

	defer func() {
		log.InfoContext(ctx, "Stopping daemon")
		C.DaemonStop()
	}()

	config, err := waitForVnetConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "Received VNet config",
		"service_credential_path", config.ServiceCredentialPath,
		"client_application_service_addr", config.ClientApplicationServiceAddr,
	)

	return trace.Wrap(workFn(ctx, config))
}

func waitForVnetConfig(ctx context.Context) (Config, error) {
	const waitForVnetConfigTimeout = 20 * time.Second
	ctx, cancel := context.WithTimeoutCause(ctx, waitForVnetConfigTimeout,
		errors.New("did not receive the VNet config within the timeout"))
	defer cancel()

	var config Config
	errC := make(chan error, 1)

	go func() {
		var result C.VnetConfigResult
		defer func() {
			C.free(unsafe.Pointer(result.error_description))
			C.free(unsafe.Pointer(result.service_credential_path))
			C.free(unsafe.Pointer(result.client_application_service_addr))
		}()

		// This call gets unblocked when the daemon gets stopped through C.DaemonStop.
		C.WaitForVnetConfig(&result)
		if !result.ok {
			errC <- trace.Wrap(errors.New(C.GoString(result.error_description)))
			return
		}

		config = Config{
			ServiceCredentialPath:        C.GoString(result.service_credential_path),
			ClientApplicationServiceAddr: C.GoString(result.client_application_service_addr),
		}
		errC <- nil
	}()

	select {
	case <-ctx.Done():
		return config, trace.Wrap(context.Cause(ctx))
	case err := <-errC:
		return config, trace.Wrap(err)
	}
}
