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

package assertserver

import (
	"context"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// AssertDeviceServerStream represents a server-side device assertion stream.
type AssertDeviceServerStream interface {
	Send(*devicepb.AssertDeviceResponse) error
	Recv() (*devicepb.AssertDeviceRequest, error)
}

// Ceremony is the server-side device assertion ceremony.
//
// Device assertion is a light form of device authentication where the user
// isn't considered and no side-effects (like certificate issuance) happen.
//
// Assertion is meant to be embedded in RPCs or streams external to the
// DeviceTrustService itself.
//
// Implementations are provided by e/.
// See e/lib/devicetrustv1.Service.CreateAssertCeremony.
type Ceremony interface {
	// AssertDevice runs the device assertion ceremonies.
	//
	// Requests and responses are consumed from the stream until the device is
	// asserted or authentication fails.
	//
	// As long as any device information is acquired from the stream, a non-nil
	// device is returned, even if the ceremony itself failed.
	AssertDevice(ctx context.Context, stream AssertDeviceServerStream) (*devicepb.Device, error)
}
