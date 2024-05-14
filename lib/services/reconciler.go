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

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
)

// ReconcilerConfig is the resource reconciler configuration.
type ReconcilerConfig[T any] struct {
	// Matcher is used to match resources.
	Matcher Matcher[T]
	// GetCurrentResources returns currently registered resources. Note that the
	// map keys must be consistent across the current and new resources.
	GetCurrentResources func() map[string]T
	// GetNewResources returns resources to compare current resources against.
	// Note that the map keys must be consistent across the current and new
	// resources.
	GetNewResources func() map[string]T
	// OnCreate is called when a new resource is detected.
	OnCreate func(context.Context, T) error
	// OnUpdate is called when an existing resource is updated.
	OnUpdate func(ctx context.Context, new, old T) error
	// OnDelete is called when an existing resource is deleted.
	OnDelete func(context.Context, T) error
	// Log is the reconciler's logger.
	Log logrus.FieldLogger
}

// Matcher is used by reconciler to match resources.
type Matcher[T any] func(T) bool

// CheckAndSetDefaults validates the reconciler configuration and sets defaults.
func (c *ReconcilerConfig[T]) CheckAndSetDefaults() error {
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
	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, "reconciler")
	}
	return nil
}

// NewReconciler creates a new reconciler with provided configuration.
func NewReconciler[T any](cfg ReconcilerConfig[T]) (*Reconciler[T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Reconciler[T]{
		cfg: cfg,
		// We do a WithFields here to force this into a *logrus.Entry, which has the ability to
		// log at the Trace level. If we were to change this in ReconcilerConfig, we'd have to
		// refactor existing code to use *logrus.Entry instead of logrus.FieldLogger, and with
		// the eventual change to slog, it seems easier to do this for now until this can be
		// changed to slog.
		log: cfg.Log.WithFields(nil),
	}, nil
}

// Reconciler reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
//
// It's used in combination with watchers by agents (app, database, desktop)
// to enable dynamically registered resources.
type Reconciler[T any] struct {
	cfg ReconcilerConfig[T]
	log *logrus.Entry
}

// Reconcile reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
func (r *Reconciler[T]) Reconcile(ctx context.Context) error {
	currentResources := r.cfg.GetCurrentResources()
	newResources := r.cfg.GetNewResources()

	r.log.Debugf("Reconciling %v current resources with %v new resources.",
		len(currentResources), len(newResources))

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
func (r *Reconciler[T]) processRegisteredResource(ctx context.Context, newResources map[string]T, name string, registered T) error {
	// See if this registered resource is still present among "new" resources.
	if _, ok := newResources[name]; ok {
		return nil
	}

	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	r.log.Infof("%v %v removed, deleting.", kind, name)
	if err := r.cfg.OnDelete(ctx, registered); err != nil {
		return trace.Wrap(err, "failed to delete  %v %v", kind, name)
	}

	return nil
}

// processNewResource checks the provided new resource agsinst currently
// registered resources.
func (r *Reconciler[T]) processNewResource(ctx context.Context, currentResources map[string]T, name string, newT T) error {
	// First see if the resource is already registered and if not, whether it
	// matches the selector labels and should be registered.
	registered, ok := currentResources[name]
	if !ok {
		kind, err := types.GetKind(newT)
		if err != nil {
			return trace.Wrap(err)
		}
		if r.cfg.Matcher(newT) {
			r.log.Infof("%v %v matches, creating.", kind, name)
			if err := r.cfg.OnCreate(ctx, newT); err != nil {
				return trace.Wrap(err, "failed to create %v %v", kind, name)
			}
			return nil
		}
		r.log.Debugf("%v %v doesn't match, not creating.", kind, name)
		return nil
	}

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
		r.log.Warnf("%v has different origin (%v vs %v), not updating.", name, newOrigin, registeredOrigin)
		return nil
	}

	// If the resource is already registered but was updated, see if its
	// labels still match.
	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	if CompareResources(newT, registered) != Equal {
		if r.cfg.Matcher(newT) {
			r.log.Infof("%v %v updated, updating.", kind, name)
			if err := r.cfg.OnUpdate(ctx, newT, registered); err != nil {
				return trace.Wrap(err, "failed to update %v %v", kind, name)
			}
			return nil
		}
		r.log.Infof("%v %v updated and no longer matches, deleting.", kind, name)
		if err := r.cfg.OnDelete(ctx, registered); err != nil {
			return trace.Wrap(err, "failed to delete %v %v", kind, name)
		}
		return nil
	}

	r.log.Tracef("%v %v is already registered.", kind, name)
	return nil
}
