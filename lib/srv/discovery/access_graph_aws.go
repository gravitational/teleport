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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	// batchSize is the maximum number of resources to send in a single
	// request to the access graph service.
	batchSize = 500
	// defaultPollInterval is the default interval between polling for access graph resources
	defaultPollInterval = 15 * time.Minute
	// Configure health check service to monitor access graph service and
	// automatically reconnect if the connection is lost without
	// relying on new events from the auth server to trigger a reconnect.
	serviceConfig = `{
		 "loadBalancingConfig": [{"round_robin": {}}],
		 "healthCheckConfig": {
			 "serviceName": ""
		 }
	 }`
)

// errNoAccessGraphFetchers is returned when there are no TAG fetchers.
var errNoAccessGraphFetchers = errors.New("no Access Graph fetchers")

func (s *Server) reconcileAccessGraph(ctx context.Context, currentTAGResources *aws_sync.Resources, stream accessgraphv1alpha.AccessGraphService_AWSEventsStreamClient, features aws_sync.Features) error {
	type fetcherResult struct {
		result *aws_sync.Resources
		err    error
	}

	allFetchers := s.getAllAWSSyncFetchers()
	if len(allFetchers) == 0 {
		// If there are no fetchers, we don't need to continue.
		// We will send a delete request for all resources and return.
		upsert, toDel := aws_sync.ReconcileResults(currentTAGResources, &aws_sync.Resources{})

		if err := push(stream, upsert, toDel); err != nil {
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

	resultsC := make(chan fetcherResult, len(allFetchers))
	// Use a channel to limit the number of concurrent fetchers.
	tokens := make(chan struct{}, 3)
	accountIds := map[string]struct{}{}
	for _, fetcher := range allFetchers {
		accountIds[fetcher.GetAccountID()] = struct{}{}
		tokens <- struct{}{}
		go func() {
			defer func() {
				<-tokens
			}()
			result, err := fetcher.Poll(ctx, features)
			resultsC <- fetcherResult{result, trace.Wrap(err)}
		}()
	}

	results := make([]*aws_sync.Resources, 0, len(allFetchers))
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
		s.Log.ErrorContext(ctx, "Error polling TAGs", "error", err)
	}
	result := aws_sync.MergeResources(results...)
	// Merge all results into a single result
	upsert, toDel := aws_sync.ReconcileResults(currentTAGResources, result)
	pushErr := push(stream, upsert, toDel)

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

	if err := s.AccessPoint.SubmitUsageEvent(s.ctx, &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_AccessGraphAwsScanEvent{
				AccessGraphAwsScanEvent: result.UsageReport(len(accountIds)),
			},
		},
	}); err != nil {
		s.Log.ErrorContext(ctx, "Error submitting usage event", "error", err)
	}

	return nil
}

// getAllAWSSyncFetchers returns all AWS sync fetchers.
func (s *Server) getAllAWSSyncFetchers() []*aws_sync.Fetcher {
	allFetchers := make([]*aws_sync.Fetcher, 0, len(s.dynamicTAGAWSFetchers))

	s.muDynamicTAGAWSFetchers.RLock()
	for _, fetcherSet := range s.dynamicTAGAWSFetchers {
		allFetchers = append(allFetchers, fetcherSet...)
	}
	s.muDynamicTAGAWSFetchers.RUnlock()

	allFetchers = append(allFetchers, s.staticTAGAWSFetchers...)
	// TODO(tigrato): submit fetchers event
	return allFetchers
}

func pushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_AWSEventsStreamClient,
	upsert *accessgraphv1alpha.AWSResourceList,
) error {
	for i := 0; i < len(upsert.Resources); i += batchSize {
		end := i + batchSize
		if end > len(upsert.Resources) {
			end = len(upsert.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.AWSEventsStreamRequest{
				Operation: &accessgraphv1alpha.AWSEventsStreamRequest_Upsert{
					Upsert: &accessgraphv1alpha.AWSResourceList{
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

func pushDeleteInBatches(
	client accessgraphv1alpha.AccessGraphService_AWSEventsStreamClient,
	toDel *accessgraphv1alpha.AWSResourceList,
) error {
	for i := 0; i < len(toDel.Resources); i += batchSize {
		end := i + batchSize
		if end > len(toDel.Resources) {
			end = len(toDel.Resources)
		}
		err := client.Send(
			&accessgraphv1alpha.AWSEventsStreamRequest{
				Operation: &accessgraphv1alpha.AWSEventsStreamRequest_Delete{
					Delete: &accessgraphv1alpha.AWSResourceList{
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

func push(
	client accessgraphv1alpha.AccessGraphService_AWSEventsStreamClient,
	upsert *accessgraphv1alpha.AWSResourceList,
	toDel *accessgraphv1alpha.AWSResourceList,
) error {
	err := pushUpsertInBatches(client, upsert)
	if err != nil {
		return trace.Wrap(err)
	}
	err = pushDeleteInBatches(client, toDel)
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.Send(
		&accessgraphv1alpha.AWSEventsStreamRequest{
			Operation: &accessgraphv1alpha.AWSEventsStreamRequest_Sync{},
		},
	)
	return trace.Wrap(err)
}

// NewAccessGraphClient returns a new access graph service client.
func newAccessGraphClient(ctx context.Context, getCert func() (*tls.Certificate, error), config AccessGraphConfig, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opt, err := grpcCredentials(config, getCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opts = append(opts,
		opt,
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
	)

	conn, err := grpc.DialContext(ctx, config.Addr, opts...)
	return conn, trace.Wrap(err)
}

// errTAGFeatureNotEnabled is returned when the TAG feature is not enabled
// in the cluster features.
var errTAGFeatureNotEnabled = errors.New("TAG feature is not enabled")

// initializeAndWatchAccessGraph creates a new access graph service client and
// watches the connection state. If the connection is closed, it will
// automatically try to reconnect.
func (s *Server) initializeAndWatchAccessGraph(ctx context.Context, reloadCh <-chan struct{}) error {
	const (
		// aws discovery semaphore lock.
		semaphoreName = "access_graph_aws_sync"
	)

	clusterFeatures := s.Config.ClusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(errTAGFeatureNotEnabled)
	}

	const (
		semaphoreExpiration = time.Minute
	)
	// AcquireSemaphoreLock will retry until the semaphore is acquired.
	// This prevents multiple discovery services to push AWS resources in parallel.
	// lease must be released to cleanup the resource in auth server.
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
	var wg sync.WaitGroup
	defer wg.Wait()

	// once the lease parent context is canceled, the lease will be released.
	// this will stop the access graph sync.
	ctx, cancel := context.WithCancel(lease)
	defer cancel()

	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "Error cleaning up semaphore", "error", err)
		}
	}()

	config := s.Config.AccessGraphConfig

	accessGraphConn, err := newAccessGraphClient(
		ctx,
		s.GetClientCert,
		config,
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	// Close the connection when the function returns.
	defer accessGraphConn.Close()
	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	stream, err := client.AWSEventsStream(ctx)
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get access graph service stream", "error", err)
		return trace.Wrap(err)
	}
	header, err := stream.Header()
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get access graph service stream header", "error", err)
		return trace.Wrap(err)
	}
	const (
		supportedResourcesKey = "supported-kinds"
	)
	supportedKinds := header.Get(supportedResourcesKey)
	if len(supportedKinds) == 0 {
		return trace.BadParameter("access graph service did not return supported kinds")
	}
	features := aws_sync.BuildFeatures(supportedKinds...)

	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.Log.InfoContext(ctx, "Access graph service connection was closed")
		}
	}()

	// Configure the poll interval
	tickerInterval := defaultPollInterval
	if s.Config.Matchers.AccessGraph != nil {
		if s.Config.Matchers.AccessGraph.PollInterval > defaultPollInterval {
			tickerInterval = s.Config.Matchers.AccessGraph.PollInterval
		} else {
			s.Log.WarnContext(ctx,
				"Access graph service poll interval cannot be less than the default",
				"default_poll_interval",
				defaultPollInterval)
		}
	}
	s.Log.InfoContext(ctx, "Access graph service poll interval", "poll_interval", tickerInterval)

	currentTAGResources := &aws_sync.Resources{}
	timer := time.NewTimer(tickerInterval)
	defer timer.Stop()
	for {
		err := s.reconcileAccessGraph(ctx, currentTAGResources, stream, features)
		if errors.Is(err, errNoAccessGraphFetchers) {
			// no fetchers, no need to continue.
			// we will wait for the config to change and re-evaluate the fetchers
			// before starting the sync.
			_, err := stream.CloseAndRecv() /* signal the end of the stream  and wait for the response */
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return trace.Wrap(err)
		}
		if !timer.Stop() {
			select {
			case <-timer.C: // drain
			default:
			}
		}
		timer.Reset(tickerInterval)
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-timer.C:
		case <-reloadCh:
		}
	}
}

// grpcCredentials returns a grpc.DialOption configured with TLS credentials.
func grpcCredentials(config AccessGraphConfig, getCert func() (*tls.Certificate, error)) (grpc.DialOption, error) {
	var pool *x509.CertPool
	if len(config.CA) > 0 {
		pool = x509.NewCertPool()
		if !pool.AppendCertsFromPEM(config.CA) {
			return nil, trace.BadParameter("failed to append CA certificate to pool")
		}
	}

	// TODO(espadolini): this doesn't honor the process' configured ciphersuites
	tlsConfig := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			tlsCert, err := getCert()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return tlsCert, nil
		},
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: config.Insecure,
		RootCAs:            pool,
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

func (s *Server) initTAGAWSWatchers(ctx context.Context, cfg *Config) error {
	fetchers, err := s.accessGraphAWSFetchersFromMatchers(ctx, cfg.Matchers, "" /* discoveryConfigName */)
	if err != nil {
		s.Log.ErrorContext(ctx, "Error initializing access graph fetchers", "error", err)
	}
	s.staticTAGAWSFetchers = fetchers

	if cfg.AccessGraphConfig.Enabled {
		go func() {
			reloadCh := s.newDiscoveryConfigChangedSub()
			for {
				allFetchers := s.getAllAWSSyncFetchers()
				// If there are no fetchers, we don't need to start the access graph sync.
				// We will wait for the config to change and re-evaluate the fetchers
				// before starting the sync.
				if len(allFetchers) == 0 {
					s.Log.DebugContext(ctx, "No AWS sync fetchers configured. Access graph sync will not be enabled.")
					select {
					case <-ctx.Done():
						return
					case <-reloadCh:
						// if the config changes, we need to re-evaluate the fetchers.
					}
					continue
				}
				// reset the currentTAGResources to force a full sync
				if err := s.initializeAndWatchAccessGraph(ctx, reloadCh); errors.Is(err, errTAGFeatureNotEnabled) {
					s.Log.WarnContext(ctx, "Access Graph specified in config, but the license does not include Teleport Identity Security. Access graph sync will not be enabled.")
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
		go func() {
			for {
				// If there are no fetchers, we don't need to start the access graph sync.
				// We will wait for the config to change and re-evaluate the fetchers
				// before starting the sync.
				if s.Matchers.AccessGraph == nil || len(s.Matchers.AccessGraph.AWS) == 0 {
					s.Log.DebugContext(ctx, "No AWS sync fetchers configured. Access graph sync will not be enabled.")
					return
				}

				var matchers []*types.AccessGraphAWSSync
				for _, matcher := range s.Matchers.AccessGraph.AWS {
					if matcher.EnableCloudTrailPolling {
						matchers = append(matchers, matcher)
					}
				}
				if len(matchers) == 0 {
					s.Log.DebugContext(ctx, "No AWS sync fetchers configured. Access graph sync will not be enabled.")
					return
				}
				if len(matchers) > 1 {
					s.Log.WarnContext(ctx, "Multiple AWS sync fetchers configured. Only the first one will be used.")
					return
				}
				// reset the currentTAGResources to force a full sync
				endTime := time.Now()
				startTime := endTime.Add(-20 * 24 * time.Hour)
				if err := s.startCloudtrailPoller(ctx, startTime, endTime, matchers[0]); errors.Is(err, errTAGFeatureNotEnabled) {
					s.Log.WarnContext(ctx, "Access Graph specified in config, but the license does not include Teleport Identity Security. Access graph sync will not be enabled.")
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

	}
	return nil
}

// accessGraphAWSFetchersFromMatchers converts Matchers into a set of AWS Sync Fetchers.
func (s *Server) accessGraphAWSFetchersFromMatchers(ctx context.Context, matchers Matchers, discoveryConfigName string) ([]*aws_sync.Fetcher, error) {
	var fetchers []*aws_sync.Fetcher
	var errs []error
	if matchers.AccessGraph == nil {
		return fetchers, nil
	}

	for _, awsFetcher := range matchers.AccessGraph.AWS {
		var assumeRole *aws_sync.AssumeRole
		if awsFetcher.AssumeRole != nil {
			assumeRole = &aws_sync.AssumeRole{
				RoleARN:    awsFetcher.AssumeRole.RoleARN,
				ExternalID: awsFetcher.AssumeRole.ExternalID,
			}
		}
		fetcher, err := aws_sync.NewFetcher(
			ctx,
			aws_sync.Config{
				AWSConfigProvider:   s.AWSConfigProvider,
				GetEKSClient:        s.GetAWSSyncEKSClient,
				GetEC2Client:        s.GetEC2Client,
				AssumeRole:          assumeRole,
				Regions:             awsFetcher.Regions,
				Integration:         awsFetcher.Integration,
				DiscoveryConfigName: discoveryConfigName,
			},
		)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fetchers = append(fetchers, fetcher)
	}

	return fetchers, trace.NewAggregate(errs...)
}

func (s *Server) startCloudtrailPoller(ctx context.Context, startDate time.Time, endDate time.Time, matcher *types.AccessGraphAWSSync) error {
	accountID, err := s.getAccountId(ctx, matcher)
	if err != nil {
		return trace.Wrap(err)
	}
	const (
		// aws discovery semaphore lock.
		semaphoreName = "access_graph_aws_cloudtrail_sync"
	)

	clusterFeatures := s.Config.ClusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(errTAGFeatureNotEnabled)
	}

	const (
		semaphoreExpiration = time.Minute
	)
	// AcquireSemaphoreLock will retry until the semaphore is acquired.
	// This prevents multiple discovery services to push AWS resources in parallel.
	// lease must be released to cleanup the resource in auth server.
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
	var wg sync.WaitGroup
	defer wg.Wait()

	// once the lease parent context is canceled, the lease will be released.
	// this will stop the access graph sync.
	ctx, cancel := context.WithCancel(lease)
	defer cancel()

	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "Error cleaning up semaphore", "error", err)
		}
	}()

	config := s.Config.AccessGraphConfig

	accessGraphConn, err := newAccessGraphClient(
		ctx,
		s.GetClientCert,
		config,
		grpc.WithDefaultServiceConfig(serviceConfig),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	// Close the connection when the function returns.
	defer accessGraphConn.Close()
	client := accessgraphv1alpha.NewAccessGraphServiceClient(accessGraphConn)

	stream, err := client.AWSCloudTrailStream(ctx)
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get access graph service stream", "error", err)
		return trace.Wrap(err)
	}
	err = stream.Send(
		&accessgraphv1alpha.AWSCloudTrailStreamRequest{
			Action: &accessgraphv1alpha.AWSCloudTrailStreamRequest_Config{
				Config: &accessgraphv1alpha.AWSCloudTrailConfig{
					StartDate: timestamppb.New(startDate),
					EndDate:   timestamppb.New(endDate),
					Regions:   matcher.Regions,
				},
			},
		},
	)
	if err != nil {
		err = consumeTillErr(stream)
		s.Log.ErrorContext(ctx, "Failed to send access graph config", "error", err)
		return trace.Wrap(err)
	}

	tagAWSConfig, err := stream.Recv()
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get aws cloud trail config", "error", err)
		return trace.Wrap(err)
	}

	if tagAWSConfig.GetCloudTrailConfig() == nil {
		return trace.BadParameter("access graph service did not return cloud trail config")
	}

	s.Log.InfoContext(ctx, "Access graph service cloud trail config", "config", tagAWSConfig.GetCloudTrailConfig())

	resumeState, err := stream.Recv()
	if err != nil {
		s.Log.ErrorContext(ctx, "Failed to get aws cloud trail resume state", "error", err)
		return trace.Wrap(err)
	}

	if resumeState.GetResumeState() == nil {
		return trace.BadParameter("access graph service did not return cloud trail resume state")
	}
	s.Log.InfoContext(ctx, "Access graph service cloud trail resume state", "resume_state", resumeState.GetResumeState())

	// Start a goroutine to watch the access graph service connection state.
	// If the connection is closed, cancel the context to stop the event watcher
	// before it tries to send any events to the access graph service.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
			s.Log.InfoContext(ctx, "Access graph service connection was closed")
		}
	}()

	// Configure the poll interval
	tickerInterval := 10 * time.Second
	s.Log.InfoContext(ctx, "Access graph service poll interval", "poll_interval", tickerInterval)

	timer := time.NewTimer(tickerInterval)
	defer timer.Stop()
	eventsC := make(chan eventChannelPayload, 100)
	state := map[string]cursor{}
	for region, regionState := range resumeState.GetResumeState().GetRegionsState() {
		state[region] = cursor{
			nextPage:      regionState.GetNextPage(),
			lastEventId:   regionState.GetLastEventId(),
			lastEventTime: regionState.GetLastEventTime().AsTime(),
		}
	}
	if err = s.pollCloudTrail(ctx,
		accountID,
		resumeState.GetResumeState().GetStartDate().AsTime(),
		resumeState.GetResumeState().GetEndDate().AsTime(),
		state,
		&wg,
		eventsC,
		matcher,
	); err != nil {
		return trace.Wrap(err)
	}

	var size int
	var events []*accessgraphv1alpha.AWSCloudTrailEvent
	const maxSize = 3 * 1024 * 1024 // 3MB
	for {
		sendData := false
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-timer.C:
			sendData = len(events) > 0
		case evts := <-eventsC:
			for _, event := range evts.events {
				size += gproto.Size(event)
			}
			state[evts.region] = evts.cursor
			events = append(events, evts.events...)

			sendData = size >= maxSize
		}

		if sendData {
			// send the events to the access graph service
			err := stream.Send(
				&accessgraphv1alpha.AWSCloudTrailStreamRequest{
					Action: &accessgraphv1alpha.AWSCloudTrailStreamRequest_Events{
						Events: &accessgraphv1alpha.AWSCloudTrailEvents{
							Events: events,
							ResumeState: stateToProtoState(
								resumeState.GetResumeState().GetStartDate().AsTime(),
								resumeState.GetResumeState().GetEndDate().AsTime(),
								state),
						},
					},
				},
			)
			if err != nil {
				err = consumeTillErr(stream)
				s.Log.ErrorContext(ctx, "Failed to send access graph service events", "error", err)
				return trace.Wrap(err)
			}
			// reset the events and size
			events = events[:0]
			size = 0
			if !timer.Stop() {
				select {
				case <-timer.C: // drain
				default:
				}
			}
			timer.Reset(tickerInterval)
		}
	}
}

func (s *Server) pollCloudTrail(ctx context.Context,
	accountID string,
	startDate time.Time,
	endDate time.Time,
	state map[string]cursor,
	wg *sync.WaitGroup,
	eventsC chan<- eventChannelPayload,
	matcher *types.AccessGraphAWSSync) error {
	// Create a new CloudTrail client for each region.
	// This is because the CloudTrail client is not thread-safe and
	// we need to create a new client for each goroutine.
	// We will use the same credentials for all clients.
	// The credentials are set in the AWSConfigProvider.
	wg.Add(len(state))
	opts := []awsconfig.OptionsFn{
		awsconfig.WithCredentialsMaybeIntegration(matcher.Integration),
	}
	if matcher.AssumeRole != nil {
		opts = append(opts, awsconfig.WithAssumeRole(matcher.AssumeRole.RoleARN, matcher.AssumeRole.ExternalID))
	}
	for region, regionState := range state {

		// Create a new goroutine for each region.
		go func(region string, regionState cursor) {
			defer wg.Done()

			awsCfg, err := s.AWSConfigProvider.GetConfig(ctx, region, opts...)
			if err != nil {
				s.Log.ErrorContext(ctx, "Error getting AWS config", "error", err)
				return
			}
			cloudTrailClient := cloudtrail.NewFromConfig(awsCfg)
			// Create a new cursor for each region.
			lastCursor := regionState
			timer := s.clock.NewTimer(time.Second)
			// Poll the CloudTrail for events in the region.
			for {
				hasNextPage, err := s.pollCloudTrailForRegion(ctx, accountID, cloudTrailClient, startDate, endDate, lastCursor, region, eventsC)
				if err != nil {
					s.Log.ErrorContext(ctx, "Error polling cloud trail", "error", err)
				}

				wait := 500 * time.Millisecond
				if !hasNextPage || err != nil {
					// If there are no more pages, we can wait for the next poll interval.
					wait = 30 * time.Second
				}

				// Wait for the next poll interval.
				if !timer.Stop() {
					select {
					case <-timer.Chan(): // drain
					default:
					}
				}
				timer.Reset(wait)
				select {
				case <-ctx.Done():
					s.Log.InfoContext(ctx, "Stopping cloud trail poller for region", "region", region)
					return
				case <-timer.Chan():
					// continue to the next iteration
				}
			}
		}(region, regionState)
	}
	return nil
}

func (s *Server) pollCloudTrailForRegion(ctx context.Context,
	accountID string,
	client *cloudtrail.Client,
	startTime time.Time,
	endTime time.Time,
	lastCursor cursor,
	region string,
	eventsC chan<- eventChannelPayload,
) (bool, error) {
	var nextToken *string
	if lastCursor.nextPage != "" {
		nextToken = aws.String(lastCursor.nextPage)
	}

	input := &cloudtrail.LookupEventsInput{
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		NextToken:  nextToken,
		MaxResults: aws.Int32(50), /* max results per page */
	}
	var events []*accessgraphv1alpha.AWSCloudTrailEvent

	for numPage := 0; numPage < 10; numPage++ {
		resp, err := client.LookupEvents(ctx, input)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if len(lastCursor.lastEventId) > 0 {
			idx := 0
			// if we have a lastEventId, we need to skip all events before it
			for i, event := range resp.Events {
				if event.EventTime.After(lastCursor.lastEventTime) {
					idx = i
					break
				}
				if aws.ToString(event.EventId) == lastCursor.lastEventId {
					idx = i + 1
					break
				}
			}
			resp.Events = resp.Events[idx:]
		}

		for _, event := range resp.Events {
			// Convert the event to a protobuf struct.
			events = append(events, convertCloudTrailEventToAccessGraphResources(event, accountID, region))
			lastCursor.lastEventTime = aws.ToTime(event.EventTime)
		}
		if len(events) > 0 {
			select {
			case <-ctx.Done():
				return false, trace.Wrap(ctx.Err())
			case eventsC <- eventChannelPayload{
				events: slices.Clone(events),
				cursor: lastCursor,
				region: region,
			}:
				events = events[:0]
			}
		}

		if resp.NextToken == nil {
			if len(resp.Events) > 0 {
				lastCursor.lastEventId = aws.ToString(resp.Events[len(resp.Events)-1].EventId)
			}
			break
		}
		// reset the lastEventID because we are going to the next page
		// and we don't want to skip the first event in the next page
		lastCursor.lastEventId = ""
		lastCursor.lastEventTime = aws.ToTime(resp.Events[len(resp.Events)-1].EventTime)
		lastCursor.nextPage = aws.ToString(resp.NextToken)

		input.NextToken = resp.NextToken

	}

	return lastCursor.lastEventId == "", nil
}

func consumeTillErr(stream accessgraphv1alpha.AccessGraphService_AWSCloudTrailStreamClient) error {
	for {
		_, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

type cursor struct {
	nextPage      string
	lastEventId   string
	lastEventTime time.Time
}

type eventChannelPayload struct {
	events []*accessgraphv1alpha.AWSCloudTrailEvent
	cursor cursor
	region string
}

func convertCloudTrailEventToAccessGraphResources(
	event cloudtrailtypes.Event,
	accountID string,
	region string,
) *accessgraphv1alpha.AWSCloudTrailEvent {

	// Convert the event to a protobuf struct.
	structEvent := &structpb.Struct{}
	if event.CloudTrailEvent != nil && len(*event.CloudTrailEvent) > 2 {
		err := protojson.UnmarshalOptions{
			DiscardUnknown: true,
		}.Unmarshal([]byte(*event.CloudTrailEvent), structEvent)
		if err != nil {
			slog.ErrorContext(context.TODO(), "Failed to unmarshal event.", "error", err)
		}
	}

	structEvent.Fields["source_region"] = structpb.NewStringValue(region)
	var resources []*accessgraphv1alpha.AWSCloudTrailEventResource
	for _, resource := range event.Resources {
		resources = append(resources, &accessgraphv1alpha.AWSCloudTrailEventResource{
			Name: aws.ToString(resource.ResourceName),
			Type: aws.ToString(resource.ResourceType),
		})
	}
	return &accessgraphv1alpha.AWSCloudTrailEvent{
		AccessKeyId:     aws.ToString(event.AccessKeyId),
		CloudTrailEvent: structEvent,
		EventId:         aws.ToString(event.EventId),
		EventName:       aws.ToString(event.EventName),
		ServiceSource:   aws.ToString(event.EventSource),
		EndTime:         timestamppb.New(aws.ToTime(event.EventTime)),
		ReadOnly:        aws.ToString(event.ReadOnly) == "true",
		Resources:       resources,
		Username:        aws.ToString(event.Username),
		AwsAccountId:    accountID,
	}
}

func stateToProtoState(startDate time.Time, endDate time.Time, state map[string]cursor) *accessgraphv1alpha.AWSCloudTrailResumeState {
	resumeState := &accessgraphv1alpha.AWSCloudTrailResumeState{
		RegionsState: make(map[string]*accessgraphv1alpha.AWSCloudTrailResumeRegionState),
		StartDate:    timestamppb.New(startDate),
		EndDate:      timestamppb.New(endDate),
	}
	for region, regionState := range state {
		var lastEventId *string
		if len(regionState.lastEventId) > 0 {
			lastEventId = aws.String(regionState.lastEventId)
		}
		resumeState.RegionsState[region] = &accessgraphv1alpha.AWSCloudTrailResumeRegionState{
			NextPage:      regionState.nextPage,
			LastEventId:   lastEventId,
			LastEventTime: timestamppb.New(regionState.lastEventTime),
		}
	}
	return resumeState
}

func (a *Server) getAccountId(ctx context.Context, matcher *types.AccessGraphAWSSync) (string, error) {
	opts := []awsconfig.OptionsFn{
		awsconfig.WithCredentialsMaybeIntegration(matcher.Integration),
	}
	if matcher.AssumeRole != nil {
		opts = append(opts, awsconfig.WithAssumeRole(matcher.AssumeRole.RoleARN, matcher.AssumeRole.ExternalID))
	}

	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because groups are global */
		opts...,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	stsClient := stsutils.NewFromConfig(awsCfg)

	input := &sts.GetCallerIdentityInput{}
	req, err := stsClient.GetCallerIdentity(ctx, input)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return aws.ToString(req.Account), nil
}
