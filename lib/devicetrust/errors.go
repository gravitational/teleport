// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package devicetrust

import (
	"errors"
	"io"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrDeviceKeyNotFound is raised for missing device key during device
// authentication.
// May be raised in situations where the binary is missing entitlements, as
// Sec/Keychain queries return empty in both cases.
// If checking for equality always use [errors.Is], as other errors may
// "impersonate" this error.
var ErrDeviceKeyNotFound = errors.New("device key not found")

// ErrPlatformNotSupported is raised for device operations attempted on
// non-supported platforms.
// trace.NotImplemented is purposefully avoided, as NotImplemented errors are
// used to detect the lack of server-side device trust support.
var ErrPlatformNotSupported = errors.New("platform not supported")

// HandleUnimplemented turns remote unimplemented errors to a more user-friendly
// error.
func HandleUnimplemented(err error) error {
	const notSupportedMsg = "device trust not supported by remote cluster"

	if errors.Is(err, io.EOF) {
		log.Debug("Device Trust: interpreting EOF as an older Teleport cluster")
		return trace.NotImplemented(notSupportedMsg)
	}

	for e := err; e != nil; {
		switch s, ok := status.FromError(e); {
		case ok && s.Code() == codes.Unimplemented:
			log.WithError(err).Debug("Device Trust: interpreting gRPC Unimplemented as OSS or older Enterprise cluster")
			return trace.NotImplemented(notSupportedMsg)
		case ok:
			return err // Unexpected status error.
		default:
			e = errors.Unwrap(e)
		}
	}
	return err
}
