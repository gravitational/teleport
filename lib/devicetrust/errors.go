/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package devicetrust

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
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
		slog.DebugContext(context.Background(), "Device Trust: interpreting EOF as an older Teleport cluster")
		return trace.NotImplemented(notSupportedMsg)
	}

	for e := err; e != nil; {
		switch s, ok := status.FromError(e); {
		case ok && s.Code() == codes.Unimplemented:
			slog.DebugContext(context.Background(), "Device Trust: interpreting gRPC Unimplemented as OSS or older Enterprise cluster", "error", err)
			return trace.NotImplemented(notSupportedMsg)
		case ok:
			return err // Unexpected status error.
		default:
			e = errors.Unwrap(e)
		}
	}
	return err
}
