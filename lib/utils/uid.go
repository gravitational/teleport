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

package utils

import (
	"github.com/google/uuid"

	"github.com/gravitational/teleport/lib/fixtures"
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
	return uuid.New().String()
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
