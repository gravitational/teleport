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

package vnet

import (
	"context"

	"github.com/gravitational/trace"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func platformAuthenticateProcess(ctx context.Context, req *vnetv1.AuthenticateProcessRequest) error {
	// The Windows service is authenticating this process, all this process has
	// to do is connect to the named pipe. The Windows service is already
	// authenticated to this process via mTLS credentials written to a
	// privileged directory.
	if err := connectToPipe(req.GetPipePath()); err != nil {
		return trace.Wrap(err, "connecting to named pipe")
	}
	log.DebugContext(ctx, "Connected to named pipe for process authentication")
	return nil
}
