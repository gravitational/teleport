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

// Package secrets implements clients for managing secret values using secret
// management tools like AWS Secrets Manager.
package secrets

import (
	"context"
	"path"
	"time"
)

const (
	// CurrentVersion is a special version string that indicates the current
	// version of the secret.
	CurrentVersion = "CURRENT"

	// PreviousVersion is a special version string that indicates the previous
	// version of the secret.
	PreviousVersion = "PREVIOUS"
)

// Secrets defines an interface for managing secrets. A secret consists of a
// key path and a list of versions that hold copies of current or past secret
// values.
type Secrets interface {
	// CreateOrUpdate creates the secret with the provided path and creates
	// first version with provided value. If secret already exists, it may try
	// to update some settings depending on the implementation and its config.
	CreateOrUpdate(ctx context.Context, key, value string) error

	// Delete deletes the secret with the provided path. All versions of the
	// secret are deleted at the same time.
	Delete(ctx context.Context, key string) error

	// PutValue creates a new secret version for the secret. CurrentVersion can
	// be provided to perform a test-and-set operation, and an error will be
	// returned if the test fails.
	PutValue(ctx context.Context, key, value, currentVersion string) error

	// GetValue returns the secret value for provided version. Besides version
	// string returned from PutValue, two specials versions "CURRENT" and
	// "PREVIOUS" can also be used to retrieve the current and previous
	// versions respectively. If the version is empty, "CURRENT" is used.
	GetValue(ctx context.Context, key, version string) (*Value, error)
}

// Value is the secret value.
type Value struct {
	// Key is the key path of the secret.
	Key string
	// Value is the value of the secret.
	Value string
	// Version is the version of the secret value.
	Version string
	// CreatedAt is the creation time of this version.
	CreatedAt time.Time
}

// DefaultKeyPrefix is the default key prefix.
const DefaultKeyPrefix = "teleport/"

// Key creates a key path with provided parts.
func Key(parts ...string) string {
	return path.Join(parts...)
}
