/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package githttp

import (
	"context"

	"github.com/gravitational/trace"
)

// UpstreamAuth decrypts service-encrypted tokens for authenticating to the
// upstream git provider.
type UpstreamAuth interface {
	// DecryptToken decrypts the service-encrypted token using the user's
	// certificate and returns a plaintext access token for the upstream git
	// provider.
	DecryptToken(ctx context.Context, userCertDER []byte, encryptedToken string) (string, error)
}

// upstreamAuth is the default UpstreamAuth implementation.
// TODO(greedy52) implement real decryption via auth server gRPC and caching.
type upstreamAuth struct{}

func (upstreamAuth) DecryptToken(context.Context, []byte, string) (string, error) {
	return "", trace.NotImplemented("token decryption not yet implemented")
}
