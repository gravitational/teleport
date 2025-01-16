/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package azuresync

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	featNamePrincipals      = "azure/principals"
	featNameRoleDefinitions = "azure/roledefinitions"
	featNameRoleAssignments = "azure/roleassignments"
	featNameVms             = "azure/virtualmachines"
)

// FetcherConcurrency is an arbitrary per-resource type concurrency to ensure significant throughput. As we increase
// the number of resource types, we may increase this value or use some other approach to fetching concurrency.
const FetcherConcurrency = 4

// Config defines parameters required for fetching resources from Azure
type Config struct {
	CloudClients        cloud.Clients
	SubscriptionID      string
	Integration         string
	DiscoveryConfigName string
}

// Resources represents the set of resources fetched from Azure
type Resources struct {
	Principals      []*accessgraphv1alpha.AzurePrincipal
	RoleDefinitions []*accessgraphv1alpha.AzureRoleDefinition
	RoleAssignments []*accessgraphv1alpha.AzureRoleAssignment
	VirtualMachines []*accessgraphv1alpha.AzureVirtualMachine
}

// Fetcher provides the functionality for fetching resources from Azure
type Fetcher struct {
	Config
	lastError               error
	lastDiscoveredResources uint64
	lastResult              *Resources

	graphClient      *msgraph.Client
	roleAssignClient RoleAssignmentsClient
	roleDefClient    RoleDefinitionsClient
	vmClient         VirtualMachinesClient
}

// NewFetcher returns a new fetcher based on configuration parameters
func NewFetcher(cfg Config, ctx context.Context) (*Fetcher, error) {
	// Establish the credential from the managed identity
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create the clients for the fetcher
	graphClient, err := msgraph.NewClient(msgraph.Config{
		TokenProvider: cred,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleAssignClient, err := azure.NewRoleAssignmentsClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleDefClient, err := azure.NewRoleDefinitionsClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vmClient, err := azure.NewVirtualMachinesClient(cfg.SubscriptionID, cred, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Fetcher{
		Config:           cfg,
		lastResult:       &Resources{},
		graphClient:      graphClient,
		roleAssignClient: roleAssignClient,
		roleDefClient:    roleDefClient,
		vmClient:         vmClient,
	}, nil
}

// Features is a set of booleans that are received from the Access Graph to indicate which resources it can receive
type Features struct {
	Principals      bool
	RoleDefinitions bool
	RoleAssignments bool
	VirtualMachines bool
}

// BuildFeatures builds the feature flags based on supported types returned by Access Graph Azure endpoints.
func BuildFeatures(values ...string) Features {
	features := Features{}
	for _, value := range values {
		switch value {
		case featNamePrincipals:
			features.Principals = true
		case featNameRoleAssignments:
			features.RoleAssignments = true
		case featNameRoleDefinitions:
			features.RoleDefinitions = true
		case featNameVms:
			features.VirtualMachines = true
		}
	}
	return features
}

// Poll fetches and deduplicates Azure resources specified by the Access Graph
func (a *Fetcher) Poll(ctx context.Context, feats Features) (*Resources, error) {
	res, err := a.fetch(ctx, feats)
	if res == nil {
		return nil, err
	}
	res.Principals = utils.DeduplicateSlice(res.Principals, azurePrincipalsKey)
	res.RoleAssignments = utils.DeduplicateSlice(res.RoleAssignments, azureRoleAssignKey)
	res.RoleDefinitions = utils.DeduplicateSlice(res.RoleDefinitions, azureRoleDefKey)
	res.VirtualMachines = utils.DeduplicateSlice(res.VirtualMachines, azureVmKey)
	return res, trace.Wrap(err)
}

// fetch returns the resources specified by the Access Graph
func (a *Fetcher) fetch(ctx context.Context, feats Features) (*Resources, error) {
	// Accumulate Azure resources
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(FetcherConcurrency)
	var result = &Resources{}
	errsCh := make(chan error, FetcherConcurrency)
	if feats.Principals {
		eg.Go(func() error {
			principals, err := fetchPrincipals(ctx, a.SubscriptionID, a.graphClient)
			if err != nil {
				errsCh <- err
				return nil
			}
			principals, err = expandMemberships(ctx, a.graphClient, principals)
			if err != nil {
				errsCh <- err
				return nil
			}
			result.Principals = principals
			return nil
		})
	}
	if feats.RoleAssignments {
		eg.Go(func() error {
			roleAssigns, err := fetchRoleAssignments(ctx, a.SubscriptionID, a.roleAssignClient)
			if err != nil {
				errsCh <- err
				return nil
			}
			result.RoleAssignments = roleAssigns
			return nil
		})
	}
	if feats.RoleDefinitions {
		eg.Go(func() error {
			roleDefs, err := fetchRoleDefinitions(ctx, a.SubscriptionID, a.roleDefClient)
			if err != nil {
				errsCh <- err
				return nil
			}
			result.RoleDefinitions = roleDefs
			return nil
		})
	}
	if feats.VirtualMachines {
		eg.Go(func() error {
			vms, err := fetchVirtualMachines(ctx, a.SubscriptionID, a.vmClient)
			if err != nil {
				errsCh <- err
				return nil
			}
			result.VirtualMachines = vms
			return nil
		})
	}

	// Return the result along with any errors collected
	_ = eg.Wait()
	close(errsCh)
	var errs []error
	for err := range errsCh {
		errs = append(errs, err)
	}
	return result, trace.NewAggregate(errs...)
}

// Status returns the number of resources last fetched and/or the last fetching/reconciling error
func (a *Fetcher) Status() (uint64, error) {
	return a.lastDiscoveredResources, a.lastError
}

// DiscoveryConfigName returns the name of the configured discovery
func (a *Fetcher) DiscoveryConfigName() string {
	return a.Config.DiscoveryConfigName
}

// IsFromDiscoveryConfig returns whether the discovery is from configuration or dynamic
func (a *Fetcher) IsFromDiscoveryConfig() bool {
	return a.Config.DiscoveryConfigName != ""
}

// GetSubscriptionID returns the ID of the Azure subscription
func (a *Fetcher) GetSubscriptionID() string {
	return a.Config.SubscriptionID
}
