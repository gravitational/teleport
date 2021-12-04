/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dbutils

import (
	"crypto/tls"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tlsca"
)

// IsDatabaseConnection inspects the TLS connection state and returns true
// if it's a database access connection as determined by the decoded
// identity from the client certificate.
func IsDatabaseConnection(state tls.ConnectionState) (bool, error) {
	// VerifiedChains must be populated after the handshake.
	if len(state.VerifiedChains) < 1 || len(state.VerifiedChains[0]) < 1 {
		return false, nil
	}
	identity, err := tlsca.FromSubject(state.VerifiedChains[0][0].Subject,
		state.VerifiedChains[0][0].NotAfter)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return identity.RouteToDatabase.ServiceName != "", nil
}
