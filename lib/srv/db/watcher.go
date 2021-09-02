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

package db

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// startWatcher starts watching changes to database resources and registers /
// unregisters the proxied databases accordingly.
func (s *Server) startWatcher(ctx context.Context) (*services.DatabaseWatcher, error) {
	log := s.log.WithField("selector", s.cfg.Selectors)
	if len(s.cfg.Selectors) == 0 {
		log.Debug("Not initializing database resource watcher.")
		return nil, nil
	}
	log.Debug("Initializing database resource watcher.")
	watcher, err := services.NewDatabaseWatcher(ctx, services.DatabaseWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: teleport.ComponentDatabase,
			Log:       log,
			Client:    s.cfg.AccessPoint,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case databases := <-watcher.DatabasesC:
				if err := s.reconcileResources(ctx, databases); err != nil {
					log.WithError(err).Errorf("Failed to reconcile %v.", databases)
				} else if s.cfg.OnReconcile != nil {
					s.cfg.OnReconcile(s.getDatabases())
				}
			case <-ctx.Done():
				log.Debug("Database resource watcher done.")
				return
			}
		}
	}()
	return watcher, nil
}

// reconcileResources reconciles the database resources this server is currently
// proxying with the provided up-to-date list of cluster databases.
func (s *Server) reconcileResources(ctx context.Context, newResources types.Databases) error {
	s.log.Debugf("Reconciling with %v database resources.", len(newResources))
	var errs []error
	// First remove databases that aren't present in the resource list anymore.
	for _, current := range s.getDatabases() {
		// Skip databases from static configuration. For backwards compatibility
		// also consider empty "origin" value.
		if current.Origin() == types.OriginConfigFile || current.Origin() == "" {
			continue
		}
		if new := newResources.Find(current.GetName()); new == nil {
			s.log.Infof("%v removed, unregistering.", current)
			if err := s.unregisterDatabase(ctx, current.GetName()); err != nil {
				errs = append(errs, trace.Wrap(err, "failed to unregister %v", current))
			}
		}
	}
	// Then add new databases if there are any or refresh those that were updated.
	for _, new := range newResources {
		if db := s.getDatabases().Find(new.GetName()); db == nil {
			// Database with this name isn't registered yet, see if it matches.
			if services.MatchDatabase(s.cfg.Selectors, new) {
				s.log.Infof("%v matches, registering.", new)
				if err := s.registerDatabase(ctx, new); err != nil {
					errs = append(errs, trace.Wrap(err, "failed to register %v", new))
				}
			} else {
				s.log.Debugf("%v doesn't match, not registering.", new)
			}
		} else if db.Origin() == types.OriginConfigFile || db.Origin() == "" {
			// Do not overwrite databases from static configuration.
			s.log.Infof("%v is part of static configuration, not registering %v.", db, new)
			continue
		} else if new.GetResourceID() != db.GetResourceID() {
			// If labels were updated, the database may no longer match.
			if services.MatchDatabase(s.cfg.Selectors, new) {
				s.log.Infof("%v updated, re-registering.", new)
				if err := s.reRegisterDatabase(ctx, new); err != nil {
					errs = append(errs, trace.Wrap(err, "failed to re-register %v", new))
				}
			} else {
				s.log.Infof("%v updated and no longer matches, unregistering.", new)
				if err := s.unregisterDatabase(ctx, new.GetName()); err != nil {
					errs = append(errs, trace.Wrap(err, "failed to unregister %v", new))
				}
			}
		} else {
			s.log.Debugf("%v is already registered.", new)
		}
	}
	return trace.NewAggregate(errs...)
}
