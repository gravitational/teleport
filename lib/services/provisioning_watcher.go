// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services

import (
	"context"

	"github.com/gravitational/trace"
)

// ProvisioningStateWatcherConfig is a OktaAssignmentWatcher configuration.
type ProvisioningStateWatcherConfig struct {
	// RWCfg is the resource watcher configuration.
	RWCfg ResourceWatcherConfig

	// PageSize is the number of Okta assignments to list at a time.
	PageSize int

	Collector ResourceCollector
}

// CheckAndSetDefaults checks parameters and sets default values.
func (cfg *ProvisioningStateWatcherConfig) CheckAndSetDefaults() error {
	if err := cfg.RWCfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// NewProvisioningStateWatcher returns a new instance of OktaAssignmentWatcher.
// The context here will be used to exit early from the resource watcher if
// needed.
func NewProvisioningStateWatcher(ctx context.Context, cfg ProvisioningStateWatcherConfig) (*ProvisioningStateWatcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	watcher, err := newResourceWatcher(ctx, cfg.Collector, cfg.RWCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ProvisioningStateWatcher{
		resourceWatcher: watcher,
	}, nil
}

// OktaAssignmentWatcher is built on top of resourceWatcher to monitor Okta assignment resources.
type ProvisioningStateWatcher struct {
	resourceWatcher *resourceWatcher
}

// Close closes the underlying resource watcher
func (o *ProvisioningStateWatcher) Close() {
	o.resourceWatcher.Close()
}

// Done returns the channel that signals watcher closer.
func (o *ProvisioningStateWatcher) Done() <-chan struct{} {
	return o.resourceWatcher.Done()
}
