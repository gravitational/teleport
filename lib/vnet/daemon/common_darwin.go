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

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=11.0
// #cgo LDFLAGS: -framework Foundation -framework ServiceManagement
// #include "common_darwin.h"
import "C"

import (
	"errors"
)

var (
	// vnetErrorDomain is a custom error domain used for Objective-C errors that pertain to VNet.
	vnetErrorDomain = C.GoString(C.VNEErrorDomain)

	// errorCodeAlreadyRunning is returned within [vnetErrorDomain] errors to indicate that the daemon
	// received a message to start after it was already running.
	errorCodeAlreadyRunning = int(C.VNEAlreadyRunningError)
	errAlreadyRunning       = errors.New("VNet is already running")

	// errorCodeMissingCodeSigningIdentifiers is returned within [vnetErrorDomain] Obj-C errors and
	// transformed to [errMissingCodeSigningIdentifiers] in Go.
	errorCodeMissingCodeSigningIdentifiers = int(C.VNEMissingCodeSigningIdentifiersError)
	errMissingCodeSigningIdentifiers       = errors.New("either identifier or team identifier is missing in code signing information; is the binary signed?")
)

var (
	// nsCocoaErrorDomain is a generic error domain used in a lot of Apple's Cocoa frameworks.
	nsCocoaErrorDomain = "NSCocoaErrorDomain"

	// https://developer.apple.com/documentation/foundation/1448136-nserror_codes/nsxpcconnectioninterrupted?changes=latest_major&language=objc
	errorCodeNSXPCConnectionInterrupted = int(C.NSXPCConnectionInterrupted)
	errXPCConnectionInterrupted         = errors.New("XPC connection interrupted")

	// https://developer.apple.com/documentation/foundation/1448136-nserror_codes/nsxpcconnectioncodesigningrequirementfailure?language=objc
	errorCodeNSXPCConnectionCodeSigningRequirementFailure = int(C.NSXPCConnectionCodeSigningRequirementFailure)
	errXPCConnectionCodeSigningRequirementFailure         = errors.New("code signing requirement failed")
)
