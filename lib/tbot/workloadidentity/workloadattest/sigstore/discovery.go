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
package sigstore

import (
	"context"
	"log/slog"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/gravitational/trace"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// maxPayloads is the maximum number of verification payloads a discovery method
// will attempt to discover. It's used to prevent a DoS attack by filling the
// registry with thousands of useless signatures.
const maxPayloads = 25

// DiscoveryConfig contains configuration for the Sigstore discovery process.
type DiscoveryConfig struct {
	Logger   *slog.Logger
	Keychain authn.Keychain
}

// Discover signatures and attestations for the given image digest.
func Discover(ctx context.Context, name, digest string, cfg DiscoveryConfig) ([]*workloadidentityv1.SigstoreVerificationPayload, error) {
	repo, err := NewRepository(name, cfg.Logger, cfg.Keychain)
	if err != nil {
		return nil, trace.Wrap(err, "parsing image reference")
	}

	hash, err := v1.NewHash(digest)
	if err != nil {
		return nil, trace.Wrap(err, "parsing image digest")
	}

	type discoveryMethod interface {
		Discover(context.Context) ([]*workloadidentityv1.SigstoreVerificationPayload, error)
	}
	var payloads []*workloadidentityv1.SigstoreVerificationPayload
	for _, method := range []discoveryMethod{
		NewCosignSignatureDiscoveryMethod(repo, hash),
		NewBundleDiscoveryMethod(repo, hash),
	} {
		p, err := method.Discover(ctx)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, p...)
	}
	return payloads, nil
}
