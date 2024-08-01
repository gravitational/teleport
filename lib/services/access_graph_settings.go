/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package services

import (
	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
)

// UnmarshalAccessGraphSettings unmarshals the AccessGraphSettings resource from JSON.
func UnmarshalAccessGraphSettings(data []byte, opts ...MarshalOption) (*clusterconfigpb.AccessGraphSettings, error) {
	out, err := UnmarshalProtoResource[*clusterconfigpb.AccessGraphSettings](data, opts...)
	return out, trace.Wrap(err)
}

// MarshalAccessGraphSettings marshals the AccessGraphSettings resource to JSON.
func MarshalAccessGraphSettings(c *clusterconfigpb.AccessGraphSettings, opts ...MarshalOption) ([]byte, error) {
	bytes, err := MarshalProtoResource(c, opts...)
	return bytes, trace.Wrap(err)
}
