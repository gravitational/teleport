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

// Package types includes types that need to be passed to lib/devicetrust/authn
// by packages that should not depend on lib/devicetrust/authn.
package types

import (
	"crypto"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// CeremonyRunParams holds parameters for [lib/devicetrust/authn.(*Ceremony).Run].
type CeremonyRunParams struct {
	// DevicesClient is a client to the DeviceTrustService.
	DevicesClient devicepb.DeviceTrustServiceClient
	// Certs holds user certs to be augmented by the authn ceremony. Only the
	// SSH certificate will be forwarded, the TLS identity is part of the
	// mTLS connection.
	Certs *devicepb.UserCertificates
	// SSHSigner, if specified, will be used to prove ownership of the SSH
	// certificate subject key by signing a challenge.
	SSHSigner crypto.Signer
}
