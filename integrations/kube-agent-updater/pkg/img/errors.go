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
	"fmt"

	"github.com/docker/distribution/reference"
)

// ImageNotValidatedError indicates that the validator did not validate the image.
// It wraps the original error and implements trace.ErrorWrapper.
type ImageNotValidatedError struct {
	err           error
	image         reference.NamedTagged
	validatorName string
}

// Error returns log friendly description of an error.
func (e *ImageNotValidatedError) Error() string {
	return fmt.Sprintf("validator '%s' rejected the image '%s': %s", e.validatorName, e.image, e.err.Error())
}

// OrigError return the original error.
func (e *ImageNotValidatedError) OrigError() error {
	return e.err
}

// Unwrap unwraps the error and return the original error.
func (e *ImageNotValidatedError) Unwrap() error {
	return e.err
}

// NotValidatedError returns an ImageNotValidatedError if the error is not nil.
func NotValidatedError(name string, image reference.NamedTagged, err error) *ImageNotValidatedError {
	if err == nil {
		return nil
	}
	return &ImageNotValidatedError{err, image, name}
}
