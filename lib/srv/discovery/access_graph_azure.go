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

package discovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/azuresync"
)

// reconcileAccessGraphAzure fetches Azure resources, creates a set of resources to delete and upsert based on
// the previous fetch, and then sends the delete and upsert results to the Access Graph stream
func (s *Server) reconcileAccessGraphAzure(
	ctx context.Context,
	currentTAGResources *azuresync.Resources,
	stream accessgraphv1alpha.AccessGraphService_AzureEventsStreamClient,
	features azuresync.Features,
) error {
	type fetcherResult struct {
		result *azuresync.Resources
		err    error
	}

	// Get all the fetchers
	allFetchers := s.getAllTAGSyncAzureFetchers()
	if len(allFetchers) == 0 {
		// If there are no fetchers, we don't need to continue.
		// We will send a delete request for all resources and return.
		upsert, toDel := azuresync.ReconcileResults(currentTAGResources, &azuresync.Resources{})

		if err := azurePush(stream, upsert, toDel); err != nil {
			s.Log.ErrorContext(ctx, "Error pushing empty resources to TAGs", "error", err)
		}
		return trace.Wrap(errNoAccessGraphFetchers)
	}

	for _, fetcher := range allFetchers {
		s.tagSyncStatus.syncStarted(fetcher, s.clock.Now())
	}
	for _, discoveryConfigName := range s.tagSyncStatus.discoveryConfigs() {
		s.updateDiscoveryConfigStatus(discoveryConfigName)
	}

	// Fetch results concurrently
	resultsC := make(chan fetcherResult, len(allFetchers))
	// Restricts concurrently running fetchers to 3
	tokens := make(chan struct{}, 3)
	accountIds := map[string]struct{}{}
	for _, fetcher := range allFetchers {
		fetcher := fetcher
		accountIds[fetcher.GetSubscriptionID()] = struct{}{}
		tokens <- struct{}{}
		go func() {
			defer func() {
				<-tokens
			}()
			result, err := fetcher.Poll(ctx, features)
			resultsC <- fetcherResult{result, trace.Wrap(err)}
		}()
	}

	// Collect the results from all fetchers.
	results := make([]*azuresync.Resources, 0, len(allFetchers))
	errs := make([]error, 0, len(allFetchers))
	for i := 0; i < len(allFetchers); i++ {
		// Each fetcher can return an error and a result.
		fetcherResult := <-resultsC
		if fetcherResult.err != nil {
			errs = append(errs, fetcherResult.err)
		}
		if fetcherResult.result != nil {
			results = append(results, fetcherResult.result)
		}
	}

	// Aggregate all errors into a single error.
	err := trace.NewAggregate(errs...)
	if err != nil {
		s.Log.ErrorContext(ctx, "Error polling TAGs", "error", err)
	}
	result := azuresync.MergeResources(results...)

	// Merge all results into a single result
	upsert, toDel := azuresync.ReconcileResults(currentTAGResources, result)
	pushErr := azurePush(stream, upsert, toDel)

	for _, fetcher := range allFetchers {
		s.tagSyncStatus.syncFinished(fetcher, pushErr, s.clock.Now())
	}
	for _, discoveryConfigName := range s.tagSyncStatus.discoveryConfigs() {
		s.updateDiscoveryConfigStatus(discoveryConfigName)
	}

	if pushErr != nil {
		s.Log.ErrorContext(ctx, "Error pushing TAGs", "error", pushErr)
		return nil
	}

	// Update the currentTAGResources with the result of the reconciliation.
	*currentTAGResources = *result
	return nil
}

// azurePushUpsertInBatches upserts resources to the Access Graph in batches
func azurePushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_AzureEventsStreamClient,
	upsert *accessgraphv1alpha.AzureResourceList,
) error {
	for i := 0; i < len(upsert.Resources); i += batchSize {
		end := i + batchSize
		if end > len(upsert.Resources) {
			end = len(upsert.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.AzureEventsStreamRequest{
				Operation: &accessgraphv1alpha.AzureEventsStreamRequest_Upsert{
					Upsert: &accessgraphv1alpha.AzureResourceList{
						Resources: upsert.Resources[i:end],
					},
				},
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// azurePushDeleteInBatches deletes resources from the Access Graph in batches
func azurePushDeleteInBatches(
	client accessgraphv1alpha.AccessGraphService_AzureEventsStreamClient,
	toDel *accessgraphv1alpha.AzureResourceList,
) error {
	for i := 0; i < len(toDel.Resources); i += batchSize {
		end := i + batchSize
		if end > len(toDel.Resources) {
			end = len(toDel.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.AzureEventsStreamRequest{
				Operation: &accessgraphv1alpha.AzureEventsStreamRequest_Delete{
					Delete: &accessgraphv1alpha.AzureResourceList{
						Resources: toDel.Resources[i:end],
					},
				},
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// azurePush upserts and deletes Azure resources to/from the Access Graph
func azurePush(
	client accessgraphv1alpha.AccessGraphService_AzureEventsStreamClient,
	upsert *accessgraphv1alpha.AzureResourceList,
	toDel *accessgraphv1alpha.AzureResourceList,
) error {
	err := azurePushUpsertInBatches(client, upsert)
	if err != nil {
		return trace.Wrap(err)
	}
	err = azurePushDeleteInBatches(client, toDel)
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.Send(
		&accessgraphv1alpha.AzureEventsStreamRequest{
			Operation: &accessgraphv1alpha.AzureEventsStreamRequest_Sync{},
		},
	)
	return trace.Wrap(err)
}

// getAllTAGSyncAzureFetchers returns both static and dynamic TAG Azure fetchers
func (s *Server) getAllTAGSyncAzureFetchers() []*azuresync.Fetcher {
	allFetchers := make([]*azuresync.Fetcher, 0, len(s.dynamicTAGAzureFetchers))

	s.muDynamicTAGAzureFetchers.RLock()
	for _, fetcherSet := range s.dynamicTAGAzureFetchers {
		allFetchers = append(allFetchers, fetcherSet...)
	}
	s.muDynamicTAGAzureFetchers.RUnlock()

	allFetchers = append(allFetchers, s.staticTAGAzureFetchers...)
	return allFetchers
}

// initializeAndWatchAzureAccessGraph initializes and watches the TAG Azure stream
func (s *Server) initializeAndWatchAzureAccessGraph(ctx context.Context, reloadCh chan struct{}) error {
	// Check if the access graph is enabled
	clusterFeatures := s.Config.ClusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(errTAGFeatureNotEnabled)
	}

	// Configure the access graph semaphore for constraining multiple discovery servers
	const (
		semaphoreExpiration = time.Minute
		semaphoreName       = "access_graph_azure_sync"
	)
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.AccessPoint,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindAccessGraph,
					SemaphoreName: semaphoreName,
					MaxLeases:     1,
					Holder:        s.Config.ServerID,
				},
				Expiry: semaphoreExpiration,
				Clock:  s.clock,
			},
			Retry: retryutils.LinearConfig{
				Clock:  s.clock,
				First:  time.Second,
				Step:   semaphoreExpiration / 2,
				Max:    semaphoreExpiration,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(lease)
	defer cancel()
	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "error cleaning up semaphore", "error", err, "semaphore", semaphoreName)
		}
	}()

	// Create the access graph client
	accessGraphConn, err := newAccessGraphClient(
		ctx,
		s.GetClientCert,
		s.Config.AccessGraphConfig,
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	// Close the connection when the function returns.
	defer accessGraphConn.Close()
	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	// Create the event stream
	stream, err := client.AzureEventsStream(ctx)
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get TAG Azure service stream", "error", err)
		return trace.Wrap(err)
	}
	header, err := stream.Header()
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get TAG Azure service stream header", "error", err)
		return trace.Wrap(err)
	}
	const (
		supportedResourcesKey = "supported-kinds"
	)
	supportedKinds := header.Get(supportedResourcesKey)
	if len(supportedKinds) == 0 {
		return trace.BadParameter("TAG Azure service did not return supported kinds")
	}
	features := azuresync.BuildFeatures(supportedKinds...)

	// Cancels the context to stop the event watcher if the access graph connection fails
	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.Log.InfoContext(ctx, "access graph service connection was closed")
		}
	}()

	// Configure the poll interval
	tickerInterval := defaultPollInterval
	if s.Config.Matchers.AccessGraph != nil {
		if s.Config.Matchers.AccessGraph.PollInterval > defaultPollInterval {
			tickerInterval = s.Config.Matchers.AccessGraph.PollInterval
		} else {
			s.Log.WarnContext(ctx,
				"Access graph Azure service poll interval cannot be less than the default",
				"default_poll_interval",
				defaultPollInterval)
		}
	}
	s.Log.InfoContext(ctx, "Access graph Azure service poll interval", "poll_interval", tickerInterval)

	// Reconciles the resources as they're imported from Azure
	azureResources := &azuresync.Resources{}
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		err := s.reconcileAccessGraphAzure(ctx, azureResources, stream, features)
		if errors.Is(err, errNoAccessGraphFetchers) {
			err := stream.CloseSend()
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return trace.Wrap(err)
		}
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.C:
		case <-reloadCh:
		}
	}
}

// initTAGAzureWatchers initializes the TAG Azure watchers
func (s *Server) initTAGAzureWatchers(ctx context.Context, cfg *Config) error {
	staticFetchers, err := s.accessGraphAzureFetchersFromMatchers(cfg.Matchers, "" /* discoveryConfigName */)
	if err != nil {
		s.Log.ErrorContext(ctx, "Error initializing access graph fetchers", "error", err)
	}
	s.staticTAGAzureFetchers = staticFetchers
	if !cfg.AccessGraphConfig.Enabled {
		return nil
	}
	go func() {
		reloadCh := s.newDiscoveryConfigChangedSub()
		for {
			fetchers := s.getAllTAGSyncAzureFetchers()
			// Wait for the config to change and re-evaluate the fetchers before starting the sync.
			if len(fetchers) == 0 {
				s.Log.DebugContext(ctx, "No Azure sync fetchers configured. Access graph sync will not be enabled.")
				select {
				case <-ctx.Done():
					return
				case <-reloadCh:
					// if the config changes, we need to get the updated list of fetchers
				}
				continue
			}
			// Reset the Azure resources to force a full sync
			if err := s.initializeAndWatchAzureAccessGraph(ctx, reloadCh); errors.Is(err, errTAGFeatureNotEnabled) {
				s.Log.WarnContext(ctx, fmt.Sprintf("Access Graph specified in config, but the license does not include %s. Access graph sync will not be enabled.", teleport.FeatureNameIdentitySecurity))
				break
			} else if err != nil {
				s.Log.WarnContext(ctx, "Error initializing and watching access graph", "error", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()
	return nil
}

// accessGraphAzureFetchersFromMatchers converts matcher configuration to fetchers for Azure resource synchronization
func (s *Server) accessGraphAzureFetchersFromMatchers(
	matchers Matchers, discoveryConfigName string) ([]*azuresync.Fetcher, error) {
	var fetchers []*azuresync.Fetcher
	var errs []error
	if matchers.AccessGraph == nil {
		return fetchers, nil
	}
	for _, matcher := range matchers.AccessGraph.Azure {
		fetcherCfg := azuresync.Config{
			SubscriptionID:      matcher.SubscriptionID,
			Integration:         matcher.Integration,
			DiscoveryConfigName: discoveryConfigName,
			OIDCCredentials:     s.AccessPoint,
		}
		fetcher, err := azuresync.NewFetcher(fetcherCfg, s.ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fetchers = append(fetchers, fetcher)
	}
	return fetchers, trace.NewAggregate(errs...)
}
