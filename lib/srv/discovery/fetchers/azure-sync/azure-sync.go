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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"sync"
)

const (
	featNamePrincipals      = "azure/principals"
	featNameRoleDefinitions = "azure/roledefinitions"
	featNameRoleAssignments = "azure/roleassignments"
	featNameVms             = "azure/virtualmachines"
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
	Principals      []*accessgraphv1alpha.AzurePrincipal
	RoleDefinitions []*accessgraphv1alpha.AzureRoleDefinition
	RoleAssignments []*accessgraphv1alpha.AzureRoleAssignment
	VirtualMachines []*accessgraphv1alpha.AzureVirtualMachine
}

type RoleDefinitionsClient interface{}

type RoleAssignmentsClient interface{}

type VirtualMachinesClient interface {
	ListVirtualMachines(ctx context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error)
}

type Fetcher struct {
	Config
	lastError               error
	lastDiscoveredResources uint64
	lastResult              *Resources

	graphClient      *GraphClient
	vmClient         VirtualMachinesClient
	roleDefClient    RoleDefinitionsClient
	roleAssignClient RoleAssignmentsClient
}

func NewFetcher(cfg Config, ctx context.Context) (*Fetcher, error) {
	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	staticCred := azure.NewStaticCredential(token)

	// Create the clients
	vmClient, err := azure.NewVirtualMachinesClient(cfg.SubscriptionID, staticCred, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Fetcher{
		Config:     cfg,
		lastResult: &Resources{},
		vmClient:   vmClient,
	}, nil
}

type Features struct {
	Principals       bool
	RoleDefinitions  bool
	RoleAssignments  bool
	VirtualMachines  bool
	ManagedDatabases bool
	AKSClusters      bool
}

// BuildFeatures builds the feature flags based on supported types returned by Access Graph
// Azure endpoints.
func BuildFeatures(values ...string) Features {
	features := Features{}
	for _, value := range values {
		switch value {
		case featNameVms:
			features.VirtualMachines = true
		case featNamePrincipals:
			features.Principals = true
		case featNameRoleDefinitions:
			features.RoleDefinitions = true
		case featNameRoleAssignments:
			features.RoleAssignments = true
			features.AKSClusters = true
		}
	}
	return features
}

func (a *Fetcher) Poll(ctx context.Context, feats Features) (*Resources, error) {
	res, err := a.fetch(ctx, feats)
	if res == nil {
		return nil, err
	}
	res.VirtualMachines = common.DeduplicateSlice(res.VirtualMachines, azureVmKey)
	res.Principals = common.DeduplicateSlice(res.Principals, azureUserKey)
	res.RoleDefinitions = common.DeduplicateSlice(res.RoleDefinitions, azureRoleDefKey)
	res.RoleAssignments = common.DeduplicateSlice(res.RoleAssignments, azureRoleAssignKey)
	return res, trace.Wrap(err)
}

func (a *Fetcher) fetch(ctx context.Context, feats Features) (*Resources, error) {
	// Accumulate Azure resources
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(FetcherConcurrency)
	var result = &Resources{}
	var errs []error
	errsCh := make(chan error)
	if feats.Principals {
		eg.Go(func() error {
			principals, err := a.fetchPrincipals(ctx)
			if err != nil {
				errsCh <- err
				return err
			}
			result.Principals = principals
			return nil
		})
	}
	if feats.RoleDefinitions {
		eg.Go(func() error {
			roleDefs, err := a.fetchRoleDefinitions(ctx)
			if err != nil {
				errsCh <- err
				return err
			}
			result.RoleDefinitions = roleDefs
			return nil
		})
	}
	if feats.RoleAssignments {
		eg.Go(func() error {
			roleAssigns, err := a.fetchRoleAssignments(ctx)
			if err != nil {
				errsCh <- err
				return err
			}
			result.RoleAssignments = roleAssigns
			return nil
		})
	}
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

func (a *Fetcher) Status() (uint64, error) {
	return a.lastDiscoveredResources, a.lastError
}
func (a *Fetcher) DiscoveryConfigName() string {
	return a.Config.DiscoveryConfigName
}
func (a *Fetcher) IsFromDiscoveryConfig() bool {
	return a.Config.DiscoveryConfigName != ""
}
func (a *Fetcher) GetSubscriptionID() string {
	return a.Config.SubscriptionID
}

func ptrsToList(ptrs []*string) []string {
	strList := make([]string, len(ptrs))
	for _, ptr := range ptrs {
		strList = append(strList, *ptr)
	}
	return strList
}
