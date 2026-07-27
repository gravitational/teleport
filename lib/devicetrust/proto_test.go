// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package devicetrust_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
)

// events.OSType duplicates teleport.devicetrust.v1.OSType because gogo doesn't
// play well with protoc-gen-go. Audit event emitters convert between the two by
// numeric value, so a name or number present in only one of them silently
// mislabels the device in the audit log.
func TestOSTypeMirrorsEventsOSType(t *testing.T) {
	assert.Equal(t, devicepb.OSType_name, apievents.OSType_name,
		"teleport.devicetrust.v1.OSType and events.OSType drifted, "+
			"update api/proto/teleport/legacy/types/events/events.proto and run `make grpc`")
}
