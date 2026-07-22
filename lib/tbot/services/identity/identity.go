/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package identity

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"log/slog"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"

	apiclient "github.com/gravitational/teleport/api/client"
	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/services/identity")
	log    = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTBot)
)

// RegisterEncryptionKeyForDelegation generates an ECIES P-256 encryption
// keypair and registers the public key with the delegation service, scoped to
// the given delegation session. Returns the private key and key ID.
func RegisterEncryptionKeyForDelegation(
	ctx context.Context,
	client *apiclient.Client,
	delegationSessionID string,
	logger *slog.Logger,
) (*ecdsa.PrivateKey, string, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, "", trace.Wrap(err, "generating encryption keypair")
	}

	pubKeyDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return nil, "", trace.Wrap(err, "marshaling encryption public key")
	}

	resp, err := client.DelegationSessionServiceClient().RegisterEncryptionKey(ctx,
		&delegationv1pb.RegisterEncryptionKeyRequest{
			DelegationSessionId: delegationSessionID,
			PublicKey:           pubKeyDER,
		})
	if err != nil {
		return nil, "", trace.Wrap(err, "registering encryption key")
	}

	keyID := resp.GetEncryptionKeyId()
	logger.InfoContext(ctx, "Registered encryption key for delegation session",
		"delegation_session_id", delegationSessionID,
		"encryption_key_id", keyID,
	)

	return privKey, keyID, nil
}
