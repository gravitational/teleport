/*
Copyright 2019 Gravitational, Inc.

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

package utils

import (
	"github.com/gravitational/teleport/lib/fixtures"

	"github.com/pborman/uuid"
)

// UID provides an interface for generating unique identifiers.
type UID interface {
	// New returns a new UUID4.
	New() string
}

// realUID is a real UID generator.
type realUID struct{}

// NewRealUID returns a new real UID generator.
func NewRealUID() UID {
	return &realUID{}
}

// New generates a new UUID4.
func (u *realUID) New() string {
	return uuid.New()
}

// fakeUID is a fake UID generator used in tests.
type fakeUID struct{}

// NewFakeUID returns a new fake UID generator used in tests.
func NewFakeUID() UID {
	return &fakeUID{}
}

// New returns a fake UUID4.
func (u *fakeUID) New() string {
	return fixtures.UUID
}
