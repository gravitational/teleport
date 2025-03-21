/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package img

import (
	"context"

	"github.com/distribution/reference"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
)

type insecureValidator struct {
	name            string
	registryOptions []ociremote.Option
}

// Name returns the validator name
func (v *insecureValidator) Name() string {
	return v.name
}

// TODO: cache this to protect against registry quotas
// The image validation is only invoked when we are in a maintenance window and
// the target version is different than our current version. In regular usage we
// are called only once per update. However, Kubernetes controllers failure mode
// is usually infinite retry loop. If something fails after the image validation,
// we might get called in a loop indefinitely. To mitigate the impact of such
// failure, ValidateAndResolveDigest should cache its result.

// ValidateAndResolveDigest resolves the image digest and always return the
// image is valid. Using this validator makes you vulnerable in case of image
// registry compromise.
func (v *insecureValidator) ValidateAndResolveDigest(ctx context.Context, image reference.NamedTagged) (NamedTaggedDigested, error) {
	ref, err := NamedTaggedToDigest(image, v.registryOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	digestedImage := NewImageRef(ref.RegistryStr(), ref.RepositoryStr(), image.Tag(), digest.Digest(ref.DigestStr()))
	return digestedImage, nil
}

// NewInsecureValidator returns an img.Validator that only resolves the image
// but does not check its signature. This must not be confused with
// NewNopValidator that returns a validator that always validate without resolving.
func NewInsecureValidator(name string, keyChain authn.Keychain) Validator {
	return &insecureValidator{
		name:            name,
		registryOptions: []ociremote.Option{ociremote.WithRemoteOptions(remote.WithAuthFromKeychain(keyChain))},
	}
}
