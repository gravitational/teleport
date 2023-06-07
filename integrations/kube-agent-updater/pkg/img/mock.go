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

	"github.com/docker/distribution/reference"
	"github.com/gravitational/trace"
)

// ValidatorMock is a fake validator that return a static answer. This is used
// for testing purposes and is inherently insecure.
type ValidatorMock struct {
	name  string
	valid bool
	image NamedTaggedDigested
}

// Name returns the ValidatorMock name.
func (i ValidatorMock) Name() string {
	return i.name
}

// ValidateAndResolveDigest returns the statically defined image validation result.
func (i ValidatorMock) ValidateAndResolveDigest(_ context.Context, _ reference.NamedTagged) (NamedTaggedDigested, error) {
	if i.valid {
		return i.image, nil
	}
	return nil, trace.CompareFailed("invalid signature")
}

// NewImageValidatorMock creates a ValidatorMock
func NewImageValidatorMock(name string, valid bool, image NamedTaggedDigested) Validator {
	return ValidatorMock{
		name:  name,
		valid: valid,
		image: image,
	}
}
