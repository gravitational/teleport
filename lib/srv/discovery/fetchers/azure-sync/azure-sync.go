/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package azure_sync

import (
	"context"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"sync"
)

const FetcherConcurrency = 5

type Config struct {
	CloudClients        cloud.Clients
	SubscriptionID      string
	Regions             []string
	Integration         string
	DiscoveryConfigName string
}

type Resources struct {
	VirtualMachines []*accessgraphv1alpha.AzureVirtualMachine
	Users           []*accessgraphv1alpha.AzureUser
	RoleDefinitions []*accessgraphv1alpha.AzureRoleDefinition
	RoleAssignments []*accessgraphv1alpha.AzureRoleAssignment
}

type Features struct {
	VirtualMachines bool
	Users           bool
	RoleDefinitions bool
	RoleAssignments bool
}

type Fetcher interface {
	Poll(context.Context, Features) (*Resources, error)
	Status() (uint64, error)
	DiscoveryConfigName() string
	IsFromDiscoveryConfig() bool
	GetSubscriptionID() string
}

type azureFetcher struct {
	Config
	lastError               error
	lastDiscoveredResources uint64
	lastResult              *Resources
}

func NewAzureFetcher(cfg Config) (Fetcher, error) {
	return &azureFetcher{
		Config:     cfg,
		lastResult: &Resources{},
	}, nil
}

func (a *azureFetcher) Poll(ctx context.Context, feats Features) (*Resources, error) {
	res, err := a.fetch(ctx, feats)
	if res == nil {
		return nil, err
	}
	res.VirtualMachines = common.DeduplicateSlice(res.VirtualMachines, azureVmKey)
	res.Users = common.DeduplicateSlice(res.Users, azureUserKey)
	return res, trace.Wrap(err)
}

func (a *azureFetcher) fetch(ctx context.Context, feats Features) (*Resources, error) {
	// Accumulate Azure resources
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(FetcherConcurrency)
	var result = &Resources{}
	var errs []error
	errsCh := make(chan error)
	if feats.VirtualMachines {
		eg.Go(func() error {
			vms, err := a.fetchVirtualMachines(ctx)
			if err != nil {
				errsCh <- err
				return err
			}
			result.VirtualMachines = vms
			return nil
		})
	}
	if feats.Users {
		eg.Go(func() error {
			users, err := a.fetchUsers(ctx)
			if err != nil {
				errsCh <- err
				return err
			}
			result.Users = users
			return nil
		})
	}

	// Collect the error messages from the error channel
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()
		for {
			err, ok := <-errsCh
			if !ok {
				return
			}
			errs = append(errs, err)
		}
	}()
	_ = eg.Wait()
	close(errsCh)
	wg.Wait()
	if len(errs) > 0 {
		return result, trace.NewAggregate(errs...)
	}

	// Return the resources
	return result, nil
}

func (a *azureFetcher) Status() (uint64, error) {
	return a.lastDiscoveredResources, a.lastError
}
func (a *azureFetcher) DiscoveryConfigName() string {
	return a.Config.DiscoveryConfigName
}
func (a *azureFetcher) IsFromDiscoveryConfig() bool {
	return a.Config.DiscoveryConfigName != ""
}
func (a *azureFetcher) GetSubscriptionID() string {
	return a.Config.SubscriptionID
}
