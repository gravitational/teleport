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
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// NamedTaggedDigested is an object with a full name, a tag and a digest.
// A name and a digest are sufficient to canonically identify a layer, however
// the tag carries the version information. controller.VersionUpdater relies on
// the tag to detect what is the current version. The tag also allows users
// to determine which version is running by looking at the image reference.
// The digest ensures the container runtime will run the exact same image than
// the one currently treated.
type NamedTaggedDigested interface {
	reference.Named
	reference.Tagged
	reference.Digested
}

// Validator validates that an image reference can be safely deployed. This
// typically involves validating that the image manifest has been signed by a
// trusted third party. The Validator also returns the image
// NamedTaggedDigested, this ensures the nodes cannot run something that has
// not been validated.
type Validator interface {
	Name() string
	ValidateAndResolveDigest(ctx context.Context, image reference.NamedTagged) (NamedTaggedDigested, error)
}

// Validators is a list of Validator. They are OR-ed: if at least a Validator
// returns true, the image is validated.
type Validators []Validator

// Validate evaluates all Validators against an image reference until one
// Validator validates the image. If no Validator validates the image, it returns
// an error aggregate containing all Validator errors.
func (v Validators) Validate(ctx context.Context, image reference.NamedTagged) (NamedTaggedDigested, error) {
	var errs []error
	log := ctrllog.FromContext(ctx).V(1)

	// We don't want to be called on an empty list and return no error as the
	// absence of error indicates the image is valid.
	if len(v) == 0 {
		return nil, trace.BadParameter("no validator provided")
	}

	for _, validator := range v {
		resolvedImages, err := validator.ValidateAndResolveDigest(ctx, image)
		// There is no need to continue if we have at least a valid signature
		if err == nil {
			log.Info("Image approved by the validator", "image", image, "validator", validator.Name(), "resolvedImages", resolvedImages)
			return resolvedImages, nil
		}
		// Not logging in error because a validator rejecting an image is a legitimate outcome in
		// multi-validator setups. For example when two validators are validating for different keys.
		// Error is propagated in an aggregate only if all validators failed
		log.Info("Validator rejected the image", "image", image, "validator", validator.Name(), "reason", err)
		errs = append(errs, NotValidatedError(validator.Name(), image, err))
	}
	log.Info("No validator approved the image", "image", image)
	return nil, trace.NewAggregate(errs...)
}

type imageRef struct {
	repository struct {
		domain string
		path   string
	}
	tag    string
	digest digest.Digest
}

// String returns the string representation of the image
func (i imageRef) String() string {
	return i.Name() + ":" + i.tag + "@" + i.digest.String()
}

// Name returns the image Name (repo domain and image path)
func (i imageRef) Name() string {
	if i.repository.domain == "" {
		return i.repository.path
	}
	return i.repository.domain + "/" + i.repository.path

}

// Tag returns the image tag
func (i imageRef) Tag() string {
	return i.tag
}

// Digest returns the image digest
func (i imageRef) Digest() digest.Digest {
	return i.digest
}

// NewImageRef returns an image reference that is both Named, Tagged and
// Digested. In this setup the tag is typically superfluous, but in our case the
// tag carries the version information and makes the images friendlier for human
// operators.
func NewImageRef(domain, path, tag string, imageDigest digest.Digest) NamedTaggedDigested {
	return imageRef{
		repository: struct {
			domain string
			path   string
		}{
			domain: domain,
			path:   path,
		},
		tag:    tag,
		digest: imageDigest,
	}
}
