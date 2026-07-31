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

package jointest

import (
	"reflect"

	"github.com/gravitational/trace"

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/tlsca"
)

// ValidateScopedHostResult verifies the scoped controls returned by a
// successful host join instead of treating a nil error as sufficient coverage.
func ValidateScopedHostResult(result *joinclient.JoinResult, token *joiningv1.ScopedToken) error {
	if result == nil {
		return trace.BadParameter("join result is nil")
	}
	if result.Certs == nil {
		return trace.BadParameter("join result certificates are nil")
	}
	if token == nil {
		return trace.BadParameter("scoped token is nil")
	}

	cert, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
	if err != nil {
		return trace.Wrap(err, "parsing TLS certificate")
	}
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return trace.Wrap(err, "parsing TLS identity")
	}

	expectedScope := token.GetSpec().GetAssignedScope()
	if expectedScope != identity.AgentScope {
		return trace.CompareFailed("assigned scope mismatch: expected %q, got %q", expectedScope, identity.AgentScope)
	}

	expectedLabelHash := joining.HashImmutableLabels(token.GetSpec().GetImmutableLabels())
	if expectedLabelHash != identity.ImmutableLabelHash {
		return trace.CompareFailed("immutable label hash mismatch: expected %q, got %q", expectedLabelHash, identity.ImmutableLabelHash)
	}

	expectedLabels := token.GetSpec().GetImmutableLabels().GetSsh()
	actualLabels := result.ImmutableLabels.GetSsh()
	if !reflect.DeepEqual(expectedLabels, actualLabels) {
		return trace.CompareFailed("immutable SSH labels mismatch: expected %v, got %v", expectedLabels, actualLabels)
	}

	return nil
}
