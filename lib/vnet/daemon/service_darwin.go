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
// #cgo LDFLAGS: -framework Foundation
// #include "service_darwin.h"
import "C"

import (
	"context"
	"unsafe"

	"github.com/gravitational/trace"
)

func Start(ctx context.Context, workFn func(Config) error) error {
	log.InfoContext(ctx, "Starting daemon")
	C.DaemonStart()
	defer func() {
		log.InfoContext(ctx, "Stopping daemon")
		C.DaemonStop()
	}()

	var result C.VnetConfigResult
	defer func() {
		C.free(unsafe.Pointer(result.error_description))
		C.free(unsafe.Pointer(result.socket_path))
		C.free(unsafe.Pointer(result.ipv6_prefix))
		C.free(unsafe.Pointer(result.dns_addr))
		C.free(unsafe.Pointer(result.home_path))
	}()

	C.WaitForVnetConfig(&result)

	if !result.ok {
		return trace.Errorf(C.GoString(result.error_description))
	}

	config := Config{
		SocketPath: C.GoString(result.socket_path),
		IPv6Prefix: C.GoString(result.ipv6_prefix),
		DNSAddr:    C.GoString(result.dns_addr),
		HomePath:   C.GoString(result.home_path),
	}
	if err := config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	log.InfoContext(ctx, "Received VNet config",
		"socket_path", config.SocketPath,
		"ipv6_prefix", config.IPv6Prefix,
		"dns_addr", config.DNSAddr,
		"home_path", config.HomePath,
	)

	return trace.Wrap(workFn(config))
}
