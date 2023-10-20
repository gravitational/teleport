/*
Copyright 2023 Gravitational, Inc.

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

package img

import (
	"context"

	"github.com/distribution/reference"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
)

type insecureValidator struct {
	name string
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
	ref, err := NamedTaggedToDigest(image)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	digestedImage := NewImageRef(ref.RegistryStr(), ref.RepositoryStr(), image.Tag(), digest.Digest(ref.DigestStr()))
	return digestedImage, nil
}

// NewInsecureValidator returns an img.Validator that only resolves the image
// but does not check its signature.
func NewInsecureValidator(name string) Validator {
	return &insecureValidator{
		name: name,
	}
}
