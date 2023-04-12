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
