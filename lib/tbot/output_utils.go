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

package tbot

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
)

const renewalRetryLimit = 5

// newBotConfigWriter returns a new BotConfigWriter that writes to the given
// Destination.
func newBotConfigWriter(ctx context.Context, dest destination.Destination, subPath string) *internal.BotConfigWriter {
	return internal.NewBotConfigWriter(ctx, dest, subPath)
}

// NewClientKeyRing returns a sane client.KeyRing for the given bot identity.
func NewClientKeyRing(ident *identity.Identity, hostCAs []types.CertAuthority) (*client.KeyRing, error) {
	pk, err := keys.ParsePrivateKey(ident.PrivateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &client.KeyRing{
		KeyRingIndex: client.KeyRingIndex{
			ClusterName: ident.ClusterName,
		},
		// tbot identities use a single private key for SSH and TLS.
		SSHPrivateKey: pk,
		TLSPrivateKey: pk,
		Cert:          ident.CertBytes,
		TLSCert:       ident.TLSCertBytes,
		TrustedCerts:  authclient.AuthoritiesToTrustedCerts(hostCAs),

		// Note: these fields are never used or persisted with identity files,
		// so we won't bother to set them. (They may need to be reconstituted
		// on tsh's end based on cert fields, though.)
		KubeTLSCredentials: make(map[string]client.TLSCredential),
		DBTLSCredentials:   make(map[string]client.TLSCredential),
	}, nil
}

func writeIdentityFile(
	ctx context.Context, log *slog.Logger, keyRing *client.KeyRing, dest destination.Destination,
) error {
	return internal.WriteIdentityFile(ctx, log, keyRing, dest)
}

// writeIdentityFileTLS writes the identity file in TLS format according to the
// core identityfile.Write method. This isn't usually needed but can be
// useful when writing out TLS certificates with alternative prefix and file
// extensions for application compatibility reasons.
func writeIdentityFileTLS(
	ctx context.Context, log *slog.Logger, keyRing *client.KeyRing, dest destination.Destination,
) error {
	return internal.WriteIdentityFileTLS(ctx, log, keyRing, dest)
}

// writeTLSCAs writes the three "main" TLS CAs to disk.
// TODO(noah): This is largely a copy of templateTLSCAs. We should reconsider
// which CAs are actually worth writing for each type of service because
// it seems inefficient to write the "Database" CA for a Kubernetes output.
func writeTLSCAs(ctx context.Context, dest destination.Destination, hostCAs, userCAs, databaseCAs []types.CertAuthority) error {
	return internal.WriteTLSCAs(ctx, dest, hostCAs, userCAs, databaseCAs)
}
