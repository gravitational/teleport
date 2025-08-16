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
	"net/netip"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
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
	Logger                        *slog.Logger
	Keychain                      authn.Keychain
	AdditionalRegistries          []string
	AllowedPrivateNetworkPrefixes []string
}

// Discover signatures and attestations for the given image digest.
func Discover(ctx context.Context, image, digest string, cfg DiscoveryConfig) ([]*workloadidentityv1.SigstoreVerificationPayload, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, trace.Wrap(err, "parsing image reference")
	}

	registries := []name.Registry{ref.Context().Registry}
	for _, host := range cfg.AdditionalRegistries {
		reg, err := name.NewRegistry(host)
		if err != nil {
			return nil, trace.Wrap(err, "parsing registry host: %s", host)
		}
		registries = append(registries, reg)
	}

	hash, err := v1.NewHash(digest)
	if err != nil {
		return nil, trace.Wrap(err, "parsing image digest")
	}

	allowedPrefixes := make([]netip.Prefix, len(cfg.AllowedPrivateNetworkPrefixes))
	for idx, prefix := range cfg.AllowedPrivateNetworkPrefixes {
		nip, err := netip.ParsePrefix(prefix)
		if err != nil {
			return nil, trace.Wrap(err, "parsing allowed network prefix [%d]: %q", idx, prefix)
		}
		allowedPrefixes[idx] = nip
	}
	transport := buildSafeTransport(allowedPrefixes)

	payloads := make([]*workloadidentityv1.SigstoreVerificationPayload, 0)
	for _, reg := range registries {
		name := reg.Repo(ref.Context().RepositoryStr())

		repo, err := NewRepository(name, cfg.Logger, cfg.Keychain, transport)
		if err != nil {
			return nil, trace.Wrap(err, "constructing repository")
		}

		if regPayloads, err := discover(ctx, repo, hash); err == nil {
			payloads = append(payloads, regPayloads...)
		} else {
			cfg.Logger.WarnContext(ctx, "Failed to discover signatures from registry",
				"registry", reg.Name(),
				"image", image,
				"image_digest", digest,
				"error", err,
			)
		}
	}
	return payloads, nil
}

func discover(ctx context.Context, repo *Repository, digest v1.Hash) ([]*workloadidentityv1.SigstoreVerificationPayload, error) {
	type discoveryMethod interface {
		Discover(context.Context) ([]*workloadidentityv1.SigstoreVerificationPayload, error)
	}
	var payloads []*workloadidentityv1.SigstoreVerificationPayload
	for _, method := range []discoveryMethod{
		NewCosignSignatureDiscoveryMethod(repo, digest),
		NewBundleDiscoveryMethod(repo, digest),
	} {
		p, err := method.Discover(ctx)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, p...)
	}
	return payloads, nil
}
