/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
