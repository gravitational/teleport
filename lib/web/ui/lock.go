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

package ui

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// Lock describes a lock suitable for webapp.
type Lock struct {
	// Name is the name of this lock (uid).
	Name string `json:"name"`
	// Message is the message displayed to locked-out users.
	Message string `json:"message"`
	// Expires if set specifies when the lock ceases to be in force.
	Expires string `json:"expires"`
	// CreatedAt is the date time that the lock was created.
	CreatedAt string `json:"createdAt"`
	// CreatedBy is the username of the author of the lock.
	CreatedBy string `json:"createdBy"`
	// Target describes the set of interactions that the lock applies to.
	Targets types.LockTarget `json:"targets"`
}

// MakeLock creates a custom lock object suitable for the webapp.
func MakeLock(lock types.Lock) Lock {
	var expiresAt, createdAt string
	if lock.LockExpiry() != nil {
		expiresAt = lock.LockExpiry().Format(time.RFC3339Nano)
	}
	if !lock.CreatedAt().IsZero() {
		createdAt = lock.CreatedAt().Format(time.RFC3339Nano)
	}

	return Lock{
		Name:      lock.GetMetadata().Name,
		Message:   lock.Message(),
		Expires:   expiresAt,
		Targets:   lock.Target(),
		CreatedAt: createdAt,
		CreatedBy: lock.CreatedBy(),
	}
}

// MakeLocks makes lock objects suitable for the webapp.
func MakeLocks(locks []types.Lock) []Lock {
	uiLocks := make([]Lock, 0, len(locks))

	for _, lock := range locks {
		uiLocks = append(uiLocks, MakeLock(lock))
	}

	return uiLocks
}
