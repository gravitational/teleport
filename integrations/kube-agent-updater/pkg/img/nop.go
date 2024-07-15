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
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
)

type nopValidator struct {
	name string
}

// Name returns the validator name
func (v *nopValidator) Name() string {
	return v.name
}

// ValidateAndResolveDigest of the nopValidator does not resolve nor
// validate the image. It always says the image is valid, and returns it as-is.
// Using this validator makes you vulnerable in case of image
// registry compromise.
func (v *nopValidator) ValidateAndResolveDigest(_ context.Context, image reference.NamedTagged) (NamedTaggedDigested, error) {
	ref, err := name.NewTag(image.String())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	digestedImage := newUnresolvedImageRef(ref.RegistryStr(), ref.RepositoryStr(), image.Tag())
	return digestedImage, nil
}

// NewNopValidator returns an img.Validator that only resolves the image
// but does not check its signature.
func NewNopValidator(name string) Validator {
	return &nopValidator{
		name: name,
	}
}

// unresolvedImageRef is the insecure version of imageRef. It does not contain the
// digest, which means we cannot enforce that the image ran by Kubernetes is
// the same that was validated.
// This should only be used with the insecure validator.
type unresolvedImageRef struct {
	repository struct {
		domain string
		path   string
	}
	tag string
}

// String returns the string representation of the image
func (i unresolvedImageRef) String() string {
	return i.Name() + ":" + i.tag
}

// Name returns the image Name (repo domain and image path)
func (i unresolvedImageRef) Name() string {
	if i.repository.domain == "" {
		return i.repository.path
	}
	return i.repository.domain + "/" + i.repository.path

}

// Tag returns the image tag
func (i unresolvedImageRef) Tag() string {
	return i.tag
}

// Digest returns nothing. It's here to implement the NamedTaggedDigested interface.
func (i unresolvedImageRef) Digest() digest.Digest {
	return ""
}

// newUnresolvedImageRef returns an image reference that both Named, Tagged but not
// Digested. This is insecure because we cannot enforce that the image ran by Kubernetes is
// the same that was validated.
// This should only be used by the nopValidator.
func newUnresolvedImageRef(domain, path, tag string) NamedTaggedDigested {
	return unresolvedImageRef{
		repository: struct {
			domain string
			path   string
		}{
			domain: domain,
			path:   path,
		},
		tag: tag,
	}
}
