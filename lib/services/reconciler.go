/*
Copyright 2021 Gravitational, Inc.

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

import (
	"context"

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// ReconcilerConfig is the resource reconciler configuration.
type ReconcilerConfig struct {
	// Selectors is a list of selectors the reconciler will match resources against.
	Selectors []Selector
	// GetResources is used to fetch currently registered resources list.
	GetResources func() types.ResourcesWithLabels
	// OnCreate is called when a new resource is detected.
	OnCreate func(context.Context, types.ResourceWithLabels) error
	// OnUpdate is called when an existing resource is updated.
	OnUpdate func(context.Context, types.ResourceWithLabels) error
	// OnDelete is called when an existing resource is deleted.
	OnDelete func(context.Context, types.ResourceWithLabels) error
	// Log is the reconciler's logger.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates the reconciler configuration and sets defaults.
func (c *ReconcilerConfig) CheckAndSetDefaults() error {
	if c.GetResources == nil {
		return trace.BadParameter("missing reconciler GetResources")
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
		c.Log = logrus.WithField(trace.Component, "reconciler")
	}
	return nil
}

// NewReconciler creates a new reconciler with provided configuration.
func NewReconciler(cfg ReconcilerConfig) (*Reconciler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Reconciler{
		cfg: cfg,
		log: cfg.Log.WithField("selectors", cfg.Selectors),
	}, nil
}

// Reconciler reconciles a list of resources returned by GetResources from its
// config with a list of provided resources.
//
// It's used in combination with watchers by agents (app, database) to enable
// dynamically registered resources.
type Reconciler struct {
	cfg ReconcilerConfig
	log logrus.FieldLogger
}

// Reconcile reconciles a list of resources returned by GetResources from its
// config with newResources and calls appropriate callbacks.
func (r *Reconciler) Reconcile(ctx context.Context, newResources types.ResourcesWithLabels) error {
	r.log.Debugf("Reconciling with %v resources.", len(newResources))
	var errs []error

	// Process already registered resources to see if any of them were removed.
	for _, current := range r.cfg.GetResources() {
		if err := r.processRegisteredResource(ctx, newResources, current); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	// Add new resources if there are any or refresh those that were updated.
	for _, new := range newResources {
		if err := r.processNewResource(ctx, new); err != nil {
			errs = append(errs, trace.Wrap(err))
		}
	}

	return trace.NewAggregate(errs...)
}

// processRegisteredResource checks the specified registered resource against the
// new list of resources.
func (r *Reconciler) processRegisteredResource(ctx context.Context, newResources types.ResourcesWithLabels, registered types.ResourceWithLabels) error {
	// Skip resources marked as static as those usually come from static config.
	// For backwards compatibility also consider empty "origin" value.
	if registered.Origin() == types.OriginConfigFile || registered.Origin() == "" {
		return nil
	}

	// See if this registered resource is still present among "new" resources.
	if new := newResources.Find(registered.GetName()); new != nil {
		return nil
	}

	r.log.Infof("%v removed, deleting.", registered)
	if err := r.cfg.OnDelete(ctx, registered); err != nil {
		return trace.Wrap(err, "failed to delete %v", registered)
	}

	return nil
}

// processNewResource checks the provided new resource agsinst currently
// registered resources.
func (r *Reconciler) processNewResource(ctx context.Context, new types.ResourceWithLabels) error {
	// First see if the resource is already registered and if not, whether it
	// matches the selector labels and should be registered.
	registered := r.cfg.GetResources().Find(new.GetName())
	if registered == nil {
		if MatchResourceLabels(r.cfg.Selectors, new) {
			r.log.Infof("%v matches, creating.", new)
			if err := r.cfg.OnCreate(ctx, new); err != nil {
				return trace.Wrap(err, "failed to create %v", new)
			}
			return nil
		}
		r.log.Debugf("%v doesn't match, not creating.", new)
		return nil
	}

	// Do not overwrite static resources. Consider resources with empty
	// origin as static for backwards compatibility.
	if registered.Origin() == types.OriginConfigFile || registered.Origin() == "" {
		r.log.Infof("%v is part of static configuration, not creating %v.", registered, new)
		return nil
	}

	// If the resource is already registered but was updated, see if its
	// labels still match.
	if new.GetResourceID() != registered.GetResourceID() {
		if MatchResourceLabels(r.cfg.Selectors, new) {
			r.log.Infof("%v updated, updating.", new)
			if err := r.cfg.OnUpdate(ctx, new); err != nil {
				return trace.Wrap(err, "failed to update %v", new)
			}
			return nil
		}
		r.log.Infof("%v updated and no longer matches, deleting.", new)
		if err := r.cfg.OnDelete(ctx, registered); err != nil {
			return trace.Wrap(err, "failed to delete %v", new)
		}
		return nil
	}

	r.log.Debugf("%v is already registered.", new)
	return nil
}
