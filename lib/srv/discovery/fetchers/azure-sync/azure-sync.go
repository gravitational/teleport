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
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// fetcherConcurrency is an arbitrary per-resource type concurrency to ensure significant throughput. As we increase
// the number of resource types, we may increase this value or use some other approach to fetching concurrency.
const fetcherConcurrency = 4

// Config defines parameters required for fetching resources from Azure
type Config struct {
	// SubscriptionID is the Azure subscriptipn ID
	SubscriptionID string
	// Integration is the name of the associated Teleport integration
	Integration string
	// DiscoveryConfigName is the name of this Discovery configuration
	DiscoveryConfigName string
}

// Resources represents the set of resources fetched from Azure
type Resources struct {
	// Principals are Azure users, groups, and service principals
	Principals []*accessgraphv1alpha.AzurePrincipal
	// RoleDefinitions are Azure role definitions
	RoleDefinitions []*accessgraphv1alpha.AzureRoleDefinition
	// RoleAssignments are Azure role assignments
	RoleAssignments []*accessgraphv1alpha.AzureRoleAssignment
	// VirtualMachines are Azure virtual machines
	VirtualMachines []*accessgraphv1alpha.AzureVirtualMachine
}

// Fetcher provides the functionality for fetching resources from Azure
type Fetcher struct {
	// Config is the configuration values for this fetcher
	Config
	// lastError is the last error returned from polling
	lastError error
	// lastDiscoveredResources is the number of resources last returned from polling
	lastDiscoveredResources uint64
	// lastResult is the last set of resources returned from polling
	lastResult *Resources

	// graphClient is the MS graph client for fetching principals
	graphClient *msgraph.Client
	// roleAssignClient is the Azure client for fetching role assignments
	roleAssignClient RoleAssignmentsClient
	// roleDefClient is the Azure client for fetching role definitions
	roleDefClient RoleDefinitionsClient
	// vmClient is the Azure client for fetching virtual machines
	vmClient VirtualMachinesClient
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

const (
	featNamePrincipals      = "azure/principals"
	featNameRoleDefinitions = "azure/roledefinitions"
	featNameRoleAssignments = "azure/roleassignments"
	featNameVms             = "azure/virtualmachines"
)

// Features is a set of booleans that are received from the Access Graph to indicate which resources it can receive
type Features struct {
	// Principals indicates Azure principals can be be fetched
	Principals bool
	// RoleDefinitions indicates Azure role definitions can be fetched
	RoleDefinitions bool
	// RoleAssignments indicates Azure role assignments can be fetched
	RoleAssignments bool
	// VirtualMachines indicates Azure virtual machines can be fetched
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
func (f *Fetcher) Poll(ctx context.Context, feats Features) (*Resources, error) {
	res, err := f.fetch(ctx, feats)
	if res == nil {
		return nil, trace.Wrap(err)
	}
	res.Principals = slices.DeduplicateKey(res.Principals, azurePrincipalsKey)
	res.RoleAssignments = slices.DeduplicateKey(res.RoleAssignments, azureRoleAssignKey)
	res.RoleDefinitions = slices.DeduplicateKey(res.RoleDefinitions, azureRoleDefKey)
	res.VirtualMachines = slices.DeduplicateKey(res.VirtualMachines, azureVmKey)
	return res, trace.Wrap(err)
}

// fetch returns the resources specified by the Access Graph
func (f *Fetcher) fetch(ctx context.Context, feats Features) (*Resources, error) {
	// Accumulate Azure resources
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(fetcherConcurrency)
	var result = &Resources{}
	// we use a larger value (50) here so there is always room for any returned error to be sent to errsCh without blocking.
	errsCh := make(chan error, 50)
	if feats.Principals {
		eg.Go(func() error {
			principals, err := fetchPrincipals(ctx, f.SubscriptionID, f.graphClient)
			if err != nil {
				errsCh <- err
				return nil
			}
			principals, err = expandMemberships(ctx, f.graphClient, principals)
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
			roleAssigns, err := fetchRoleAssignments(ctx, f.SubscriptionID, f.roleAssignClient)
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
			roleDefs, err := fetchRoleDefinitions(ctx, f.SubscriptionID, f.roleDefClient)
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
			vms, err := fetchVirtualMachines(ctx, f.SubscriptionID, f.vmClient)
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
	return result, trace.NewAggregateFromChannel(errsCh, context.WithoutCancel(ctx))
}

// Status returns the number of resources last fetched and/or the last fetching/reconciling error
func (f *Fetcher) Status() (uint64, error) {
	return f.lastDiscoveredResources, f.lastError
}

// DiscoveryConfigName returns the name of the configured discovery
func (f *Fetcher) DiscoveryConfigName() string {
	return f.Config.DiscoveryConfigName
}

// IsFromDiscoveryConfig returns whether the discovery is from configuration or dynamic
func (f *Fetcher) IsFromDiscoveryConfig() bool {
	return f.Config.DiscoveryConfigName != ""
}

// GetSubscriptionID returns the ID of the Azure subscription
func (f *Fetcher) GetSubscriptionID() string {
	return f.Config.SubscriptionID
}
