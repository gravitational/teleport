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
