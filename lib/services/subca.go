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

package services

import subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"

// MarshalCertAuthorityOverride marshals a CA override resource.
func MarshalCertAuthorityOverride(resource *subcav1.CertAuthorityOverride, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(resource, opts...)
}

// UnmarshalCertAuthorityOverride unmarshals a CA override resource.
func UnmarshalCertAuthorityOverride(data []byte, opts ...MarshalOption) (*subcav1.CertAuthorityOverride, error) {
	return UnmarshalProtoResource[*subcav1.CertAuthorityOverride](data, opts...)
}
