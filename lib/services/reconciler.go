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

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Matcher is used by reconciler to match resources.
type Matcher[T any] func(T) bool

// GenericReconcilerConfig is the resource reconciler configuration that allows
// any type implementing comparable to be a key.
type GenericReconcilerConfig[K comparable, T any] struct {
	// Matcher is used to match resources.
	Matcher Matcher[T]
	// GetCurrentResources returns currently registered resources. Note that the
	// map keys must be consistent across the current and new resources.
	GetCurrentResources func() map[K]T
	// GetNewResources returns resources to compare current resources against.
	// Note that the map keys must be consistent across the current and new
	// resources.
	GetNewResources func() map[K]T
	// Compare allows custom comparators without having to implement IsEqual.
	// Defaults to `CompareResources[T]` if not specified.
	CompareResources func(T, T) int
	// OnCreate is called when a new resource is detected.
	OnCreate func(context.Context, T) error
	// OnUpdate is called when an existing resource is updated.
	OnUpdate func(ctx context.Context, new, old T) error
	// OnDelete is called when an existing resource is deleted.
	OnDelete func(context.Context, T) error
	// Logger emits log messages.
	Logger *slog.Logger
	// AllowOriginChanges is a flag that allows the reconciler to change the
	// origin value of a reconciled resource. By default, origin changes are
	// disallowed to enforce segregation between of resources from different
	// sources.
	AllowOriginChanges bool
}

// CheckAndSetDefaults validates the reconciler configuration and sets defaults.
func (c *GenericReconcilerConfig[K, T]) CheckAndSetDefaults() error {
	if c.Matcher == nil {
		return trace.BadParameter("missing reconciler Matcher")
	}
	if c.GetCurrentResources == nil {
		return trace.BadParameter("missing reconciler GetCurrentResources")
	}
	if c.GetNewResources == nil {
		return trace.BadParameter("missing reconciler GetNewResources")
	}
	if c.OnCreate == nil {
		return trace.BadParameter("missing reconciler OnCreate")
	}
	if c.OnUpdate == nil {
		return trace.BadParameter("missing reconciler OnUpdate")
	}
	if c.OnDelete == nil {
		return trace.BadParameter("missing reconciler OnDelete")
	}
	if c.CompareResources == nil {
		c.CompareResources = CompareResources[T]
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "reconciler")
	}
	return nil
}

// NewGenericReconciler creates a new GenericReconciler with provided configuration.
func NewGenericReconciler[K comparable, T any](cfg GenericReconcilerConfig[K, T]) (*GenericReconciler[K, T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &GenericReconciler[K, T]{
		cfg:    cfg,
		logger: cfg.Logger,
	}, nil
}

// GenericReconciler reconciles currently registered resources with new
// resources and creates/updates/deletes them appropriately.
//
// It's used in combination with watchers by agents (app, database, desktop)
// to enable dynamically registered resources.
type GenericReconciler[K comparable, T any] struct {
	cfg    GenericReconcilerConfig[K, T]
	logger *slog.Logger
}

// Reconcile reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
func (r *GenericReconciler[K, T]) Reconcile(ctx context.Context) error {
	currentResources := r.cfg.GetCurrentResources()
	newResources := r.cfg.GetNewResources()

	r.logger.DebugContext(ctx, "Reconciling current resources with new resources",
		"current_resource_count", len(currentResources), "new_resource_count", len(newResources))

	var errs []error

	// Process already registered resources to see if any of them were removed.
	for key, current := range currentResources {
		if err := r.processRegisteredResource(ctx, newResources, key, current); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	// Add new resources if there are any or refresh those that were updated.
	for key, newResource := range newResources {
		if err := r.processNewResource(ctx, currentResources, key, newResource); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	return trace.NewAggregate(errs...)
}

// processRegisteredResource checks the specified registered resource against the
// new list of resources.
func (r *GenericReconciler[K, T]) processRegisteredResource(ctx context.Context, newResources map[K]T, key K, registered T) error {
	// See if this registered resource is still present among "new" resources.
	if _, ok := newResources[key]; ok {
		return nil
	}

	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	r.logger.InfoContext(ctx, "Resource was removed, deleting", "name", key)
	if err := r.cfg.OnDelete(ctx, registered); err != nil {
		return trace.Wrap(err, "failed to delete  %v %v", kind, key)
	}

	return nil
}

// processNewResource checks the provided new resource against currently
// registered resources.
func (r *GenericReconciler[K, T]) processNewResource(ctx context.Context, currentResources map[K]T, key K, newT T) error {
	// First see if the resource is already registered and if not, whether it
	// matches the selector labels and should be registered.
	registered, ok := currentResources[key]
	if !ok {
		kind, err := types.GetKind(newT)
		if err != nil {
			return trace.Wrap(err)
		}
		if r.cfg.Matcher(newT) {
			r.logger.InfoContext(ctx, "New resource matches, creating", "name", key)
			if err := r.cfg.OnCreate(ctx, newT); err != nil {
				return trace.Wrap(err, "failed to create %v %v", kind, key)
			}
			return nil
		}
		r.logger.DebugContext(ctx, "New resource doesn't match, not creating", "name", key)
		return nil
	}

	if !r.cfg.AllowOriginChanges {
		// Don't overwrite resource of a different origin (e.g., keep static resource from config and ignore dynamic resource)
		registeredOrigin, err := types.GetOrigin(registered)
		if err != nil {
			return trace.Wrap(err)
		}
		newOrigin, err := types.GetOrigin(newT)
		if err != nil {
			return trace.Wrap(err)
		}
		if registeredOrigin != newOrigin {
			r.logger.WarnContext(ctx, "New resource has different origin, not updating",
				"name", key, "new_origin", newOrigin, "existing_origin", registeredOrigin)
			return nil
		}
	}

	// If the resource is already registered but was updated, see if its
	// labels still match.
	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	if r.cfg.CompareResources(newT, registered) != Equal {
		if r.cfg.Matcher(newT) {
			r.logger.InfoContext(ctx, "Existing resource updated, updating", "name", key)
			if err := r.cfg.OnUpdate(ctx, newT, registered); err != nil {
				return trace.Wrap(err, "failed to update %v %v", kind, key)
			}
			return nil
		}
		r.logger.InfoContext(ctx, "Existing resource updated and no longer matches, deleting", "name", key)
		if err := r.cfg.OnDelete(ctx, registered); err != nil {
			return trace.Wrap(err, "failed to delete %v %v", kind, key)
		}
		return nil
	}

	r.logger.Log(ctx, logutils.TraceLevel, "Existing resource is already registered", "name", key)
	return nil
}

// ReconcilerConfig holds the configuration for a reconciler
type ReconcilerConfig[T any] GenericReconcilerConfig[string, T]

// Reconciler reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
//
// This type exists for backwards compatibility, and is a simple wrapper around
// a GenericReconciler[string, T]
type Reconciler[T any] GenericReconciler[string, T]

// NewReconciler creates a new reconciler with provided configuration.
//
// Creates a new GenericReconciler[string, T] and wraps it in a Reconciler[T]
// for backwards compatibility.
func NewReconciler[T any](cfg ReconcilerConfig[T]) (*Reconciler[T], error) {
	embedded, err := NewGenericReconciler(GenericReconcilerConfig[string, T](cfg))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return (*Reconciler[T])(embedded), nil
}

// Reconcile reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
func (r *Reconciler[T]) Reconcile(ctx context.Context) error {
	return (*GenericReconciler[string, T])(r).Reconcile(ctx)
}
