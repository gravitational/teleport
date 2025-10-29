// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package join

import (
	"time"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
)

// HostCertsParams is the set of parameters used to generate host certificates
// when a host joins the cluster.
type HostCertsParams struct {
	// HostID is the unique ID of the host.
	HostID string
	// HostName is a user-friendly host name.
	HostName string
	// SystemRole is the main system role requested, e.g. Instance, Node, Proxy, etc.
	SystemRole types.SystemRole
	// AuthenticatedSystemRoles is a set of system roles that the Instance
	// identity currently re-joining has authenticated.
	AuthenticatedSystemRoles types.SystemRoles
	// PublicTLSKey is the requested TLS public key in PEM-encoded PKIX DER format.
	PublicTLSKey []byte
	// PublicSSHKey is the requested SSH public key in SSH authorized keys format.
	PublicSSHKey []byte
	// AdditionalPrincipals is a list of additional principals
	// to include in OpenSSH and X509 certificates
	AdditionalPrincipals []string
	// DNSNames is a list of DNS names to include in x509 certificates.
	DNSNames []string
	// RemoteAddr is the remote address of the host requesting a host certificate.
	RemoteAddr string
	// RawJoinClaims are raw claims asserted by specific join methods.
	RawJoinClaims any
}

// BotCertsParams is the set of parameters used to generate bot certificates
// when a bot joins the cluster.
type BotCertsParams struct {
	// PublicTLSKey is the requested TLS public key in PEM-encoded PKIX DER format.
	PublicTLSKey []byte
	// PublicSSHKey is the requested SSH public key in SSH authorized keys format.
	PublicSSHKey []byte
	// BotInstanceID is a trusted instance identifier for a Machine ID bot,
	// provided to Auth by the Join Service when bots rejoin via a client
	// certificate extension.
	BotInstanceID string
	// PreviousBotInstanceID is a trusted previous instance identifier for a
	// Machine ID bot.
	PreviousBotInstanceID string
	// BotGeneration is a trusted generation counter value for Machine ID bots,
	// provided to Auth by the Join Service when bots rejoin via a client
	// certificate extension.
	BotGeneration int32
	// Expires is a desired time of the expiry of user certificates. This only
	// applies to bot joining, and will be ignored by node joining.
	Expires *time.Time
	// RemoteAddr is the remote address of the bot requesting a bot certificate.
	RemoteAddr string
	// RawJoinClaims are raw claims asserted by specific join methods.
	RawJoinClaims any
	// Attrs is a collection of attributes that result from the join process.
	Attrs *workloadidentityv1pb.JoinAttrs
}
