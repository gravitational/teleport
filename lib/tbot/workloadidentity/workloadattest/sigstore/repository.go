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
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"

	"code.dny.dev/ssrf"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gravitational/trace"
)

// MaxLayerSize is the maximum size layer we will pull from from an OCI registry.
const MaxLayerSize = 10 << 20 // 10MiB

// Repository is a handle on a repository in an OCI registry.
type Repository struct {
	repo      name.Repository
	logger    *slog.Logger
	keychain  authn.Keychain
	transport http.RoundTripper
}

// NewRepository creates a new Repository with the given reference.
func NewRepository(repo name.Repository, logger *slog.Logger, keychain authn.Keychain, transport http.RoundTripper) (*Repository, error) {
	if keychain == nil {
		keychain = authn.DefaultKeychain
	}
	return &Repository{
		repo:      repo,
		logger:    logger,
		keychain:  keychain,
		transport: transport,
	}, nil
}

// Manifest fetches the manifest at the given reference (tag or digest) from the
// repository.
func (r *Repository) Manifest(ctx context.Context, ref Reference) (*v1.Manifest, error) {
	mfBytes, err := crane.Manifest(
		ref.String(r.repo),
		crane.WithContext(ctx),
		crane.WithAuthFromKeychain(r.keychain),
		crane.WithTransport(r.transport),
	)
	if err != nil {
		return nil, trace.Wrap(err, "fetching manifest")
	}
	mf, err := v1.ParseManifest(bytes.NewReader(mfBytes))
	if err != nil {
		return nil, trace.Wrap(err, "parsing manifest")
	}
	return mf, nil
}

// Layer pulls the layer with the given digest from the repository. If the layer
// is larger than MaxLayerSize, an error will be returned.
func (r *Repository) Layer(ctx context.Context, digest v1.Hash) ([]byte, error) {
	layer, err := crane.PullLayer(
		Digest(digest.String()).String(r.repo),
		crane.WithContext(ctx),
		crane.WithAuthFromKeychain(r.keychain),
		crane.WithTransport(r.transport),
	)
	if err != nil {
		return nil, trace.Wrap(err, "pulling layer")
	}

	// Checking the layer size makes a HEAD request to the registry. We still
	// wrap the reader in an io.LimitReader below in case a (poorly behaved)
	// registry changes the Content-Length header between the HEAD and GET
	// requests.
	size, err := layer.Size()
	if err != nil {
		return nil, trace.Wrap(err, "getting layer size")
	}
	if size > MaxLayerSize {
		return nil, trace.Errorf("layer too large: %d", size)
	}

	// Note: the layers we interact with (e.g. simple signing envelopes) are
	// written to the registry *uncompressed* so we use the the Compressed
	// method which returns the original byte stream rather than trying to
	// apply decompression.
	reader, err := layer.Compressed()
	if err != nil {
		return nil, trace.Wrap(err, "opening layer reader")
	}
	defer func() {
		if err := reader.Close(); err != nil {
			r.logger.WarnContext(ctx, "Failed to close layer reader",
				"error", err,
				"repository", r.repo.Name(),
				"digest", digest.String(),
			)
		}
	}()

	contents, err := io.ReadAll(io.LimitReader(reader, MaxLayerSize))
	if err != nil {
		return nil, trace.Wrap(err, "reading layer contents")
	}
	return contents, nil
}

// Referrers returns an index of all of the manifests linked to the given digest
// using the Referrers API. If the registry does not support the Referrers API
// it will automatically fall back to using the [tag schema] instead.
//
// [tag schema]: https://github.com/opencontainers/distribution-spec/blob/main/spec.md#referrers-tag-schema
func (r *Repository) Referrers(ctx context.Context, digest v1.Hash) (*v1.IndexManifest, error) {
	referrers, err := remote.Referrers(
		r.repo.Digest(digest.String()),
		remote.WithContext(ctx),
		remote.WithAuthFromKeychain(r.keychain),
		remote.WithTransport(r.transport),
	)
	if err != nil {
		return nil, trace.Wrap(err, "finding referrers")
	}

	indexManifest, err := referrers.IndexManifest()
	if err != nil {
		return nil, trace.Wrap(err, "unmarshaling index manifest")
	}
	return indexManifest, nil
}

// Reference represents a tag or digest reference.
type Reference interface {
	// String returns the fully-formed reference prefixed by the registry and
	// repository names.
	String(repo name.Repository) string
}

// Tag creates a tag reference with the given identifier.
func Tag(tag string) TagReference {
	return TagReference(tag)
}

// TagReference represents a reference to a given tag (e.g. `:latest`)
type TagReference string

// String satisfies the Reference interface.
func (t TagReference) String(repo name.Repository) string {
	return repo.Tag(string(t)).Name()
}

// Digest creates a digest reference with the given identifier.
func Digest(hash string) DigestReference {
	return DigestReference(hash)
}

// DigestReference represents a reference to a given digest (e.g. `@sha256:<hash>)
type DigestReference string

// String satisfies the Reference interface.
func (t DigestReference) String(repo name.Repository) string {
	return repo.Digest(string(t)).Name()
}

// buildSafeTransport builds an http.RoundTripper which refuses to access private
// network addresses by default to prevent SSRF attacks. You can override this on
// a per-address basis by passing a list of allowed prefixes (i.e. CIDR blocks).
func buildSafeTransport(allowedPrefixes []netip.Prefix) http.RoundTripper {
	var v4, v6 []netip.Prefix
	for _, prefix := range allowedPrefixes {
		if prefix.Addr().Is4() {
			v4 = append(v4, prefix)
		} else {
			v6 = append(v6, prefix)
		}
	}
	return &http.Transport{
		DialContext: (&net.Dialer{
			// By default, ssrf.New blocks all private IPv4 and IPv6 addresses
			// including the loopback addresses, RFC 1918, link local addresses,
			// broadcast addresses, and some other special cases.
			//
			// It also only allows port 80 and 443, but we relax that requirement.
			Control: ssrf.New(
				ssrf.WithAnyPort(),
				ssrf.WithAllowedV4Prefixes(v4...),
				ssrf.WithAllowedV6Prefixes(v6...),
			).Safe,
		}).DialContext,
	}
}
