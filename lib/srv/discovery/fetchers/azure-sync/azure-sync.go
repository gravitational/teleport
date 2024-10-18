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
	"golang.org/x/sync/errgroup"
)

type Config struct {
	CloudClients        cloud.Clients
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
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(5)
	var result = &Resources{}
	if feats.VirtualMachines {
		eg.Go(func() error {
			_, err := a.pollVirtualMachines(ctx)
			if err != nil {
				return err
			}
			return nil
		})
	}
	err := eg.Wait()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *azureFetcher) Status() (uint64, error) {
	return 0, nil
}
func (a *azureFetcher) DiscoveryConfigName() string {
	return ""
}
func (a *azureFetcher) IsFromDiscoveryConfig() bool {
	return false
}
func (a *azureFetcher) GetSubscriptionID() string {
	return ""
}
