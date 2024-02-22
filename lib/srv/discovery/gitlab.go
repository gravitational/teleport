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

package discovery

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/gitlab"
)

func (s *Server) reconcileGitlab(ctx context.Context, currentTAGResources *gitlab.Resources, stream accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient) {
	type fetcherResult struct {
		result *gitlab.Resources
		err    error
	}

	allFetchers := s.getAllGitlabSyncFetchers()

	resultsC := make(chan fetcherResult, len(allFetchers))
	// Use a channel to limit the number of concurrent fetchers.
	tokens := make(chan struct{}, 3)
	for _, fetcher := range allFetchers {
		fetcher := fetcher
		tokens <- struct{}{}
		go func() {
			defer func() {
				<-tokens
			}()
			result, err := fetcher.Poll(ctx)
			resultsC <- fetcherResult{result, trace.Wrap(err)}
		}()
	}

	results := make([]*gitlab.Resources, 0, len(allFetchers))
	errs := make([]error, 0, len(allFetchers))
	// Collect the results from all fetchers.
	// Each fetcher can return an error and a result.
	for i := 0; i < len(allFetchers); i++ {
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
		s.Log.WithError(err).Error("Error polling TAGs")
	}
	result := gitlab.MergeResources(results...)
	// Merge all results into a single result
	upsert, delete := gitlab.ReconcileResults(currentTAGResources, result)
	err = pushGitlab(stream, upsert, delete)
	if err != nil {
		s.Log.WithError(err).Error("Error pushing TAGs")
		return
	}
	// Update the currentTAGResources with the result of the reconciliation.
	*currentTAGResources = *result
}

// getAllGitlabSyncFetchers returns all Gitlab sync fetchers.
func (s *Server) getAllGitlabSyncFetchers() []*gitlab.GitlabFetcher {
	allFetchers := make([]*gitlab.GitlabFetcher, 0, len(s.dynamicTAGSyncFetchers))

	s.muDynamicGitlabSyncFetchers.RLock()
	for _, fetcher := range s.dynamicGitlabSyncFetchers {
		if fetcher == nil {
			continue
		}
		allFetchers = append(allFetchers, fetcher)
	}
	s.muDynamicGitlabSyncFetchers.RUnlock()

	// TODO(tigrato): submit fetchers event
	return allFetchers
}

func pushGitlabUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient,
	upsert *accessgraphv1alpha.GitlabResourceList,
) error {
	for i := 0; i < len(upsert.Resources); i += batchSize {
		end := i + batchSize
		if end > len(upsert.Resources) {
			end = len(upsert.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.GitlabEventsStreamRequest{
				Operation: &accessgraphv1alpha.GitlabEventsStreamRequest_Upsert{
					Upsert: &accessgraphv1alpha.GitlabResourceList{
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

func pushGitlabDeleteInBatches(
	client accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient,
	delete *accessgraphv1alpha.GitlabResourceList,
) error {
	for i := 0; i < len(delete.Resources); i += batchSize {
		end := i + batchSize
		if end > len(delete.Resources) {
			end = len(delete.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.GitlabEventsStreamRequest{
				Operation: &accessgraphv1alpha.GitlabEventsStreamRequest_Delete{
					Delete: &accessgraphv1alpha.GitlabResourceList{
						Resources: delete.Resources[i:end],
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

func pushGitlab(
	client accessgraphv1alpha.AccessGraphService_GitlabEventsStreamClient,
	upsert *accessgraphv1alpha.GitlabResourceList,
	delete *accessgraphv1alpha.GitlabResourceList,
) error {
	err := pushGitlabUpsertInBatches(client, upsert)
	if err != nil {
		return trace.Wrap(err)
	}
	err = pushGitlabDeleteInBatches(client, delete)
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.Send(
		&accessgraphv1alpha.GitlabEventsStreamRequest{
			Operation: &accessgraphv1alpha.GitlabEventsStreamRequest_Sync{},
		},
	)
	return trace.Wrap(err)
}

// initializeAndWatchAccessGraph creates a new access graph service client and
// watches the connection state. If the connection is closed, it will
// automatically try to reconnect.
func (s *Server) initializeAndWatchGitlabAccessGraph(ctx context.Context, reloadCh <-chan struct{}) error {
	// Configure health check service to monitor access graph service and
	// automatically reconnect if the connection is lost without
	// relying on new events from the auth server to trigger a reconnect.
	const serviceConfig = `{
		 "loadBalancingPolicy": "round_robin",
		 "healthCheckConfig": {
			 "serviceName": ""
		 }
	 }`

	config := s.Config.AccessGraphConfig

	accessGraphConn, err := newAccessGraphClient(
		ctx,
		s.ServerCredentials.Certificates,
		config,
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	// Close the connection when the function returns.
	defer accessGraphConn.Close()
	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	stream, err := client.GitlabEventsStream(ctx)
	if err != nil {
		s.Log.WithError(err).Error("Failed to get access graph service stream")
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	go func() {
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.Log.Info("access graph service connection was closed")
		}
	}()

	currentTAGResources := &gitlab.Resources{}
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		s.reconcileGitlab(ctx, currentTAGResources, stream)
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.C:
		case <-reloadCh:
		}
	}
}

// accessGraphFetchersFromMatchers converts Matchers into a set of Gitlab Sync Fetchers.
func (s *Server) gitlabaccessGraphFetchersFromMatchers(ctx context.Context, matchers Matchers) (*gitlab.GitlabFetcher, error) {
	if matchers.Gitlab == nil {
		return nil, nil
	}

	fetcher := gitlab.New(
		matchers.Gitlab.Token,
	)

	return fetcher, nil
}
