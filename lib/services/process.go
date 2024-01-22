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

package services

import "context"

// ProcessReloadContext adds a flag to the context to indicate the Teleport
// process is reloading.
func ProcessReloadContext(parent context.Context) context.Context {
	return addFlagToContext[processReloadFlag](parent)
}

// IsProcessReloading returns true if the Teleport process is reloading.
func IsProcessReloading(ctx context.Context) bool {
	return getFlagFromContext[processReloadFlag](ctx)
}

// ProcessForkedContext adds a flag to the context to indicate the Teleport
// process has running forked child(ren).
func ProcessForkedContext(parent context.Context) context.Context {
	return addFlagToContext[processForkedFlag](parent)
}

// HasProcessForked returns true if the Teleport process has running forked
// child(ren).
func HasProcessForked(ctx context.Context) bool {
	return getFlagFromContext[processForkedFlag](ctx)
}

// ShouldDeleteServerHeartbeatsOnShutdown checks whether server heartbeats
// should be deleted based on the process shutdown context.
func ShouldDeleteServerHeartbeatsOnShutdown(ctx context.Context) bool {
	switch {
	// During a reload, deregistration of the old heartbeats by the old
	// instance may race with the creation of the new heartbeats by the new
	// instance. Thus skip deleting the heartbeats to prevent them from
	// disappearing momentarily after the reload.
	case IsProcessReloading(ctx):
		return false
	// A child process can be forked to upgrade the Teleport binary. The child
	// will take over the heartbeats so do NOT delete them in that case. In
	// worst case scenarios if the child fails to register new heartbeats, the
	// old ones will get deleted automatically upon expiry.
	case HasProcessForked(ctx):
		return false
	default:
		return true
	}
}

func addFlagToContext[FlagType any](parent context.Context) context.Context {
	return context.WithValue(parent, (*FlagType)(nil), (*FlagType)(nil))
}
func getFlagFromContext[FlagType any](ctx context.Context) bool {
	_, ok := ctx.Value((*FlagType)(nil)).(*FlagType)
	return ok
}

type processReloadFlag struct{}
type processForkedFlag struct{}
