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
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"

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
	for range allFetchers {
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

func (s *Server) getAllAWSSyncFetchersWithTrailEnabled() []*types.AccessGraphAWSSync {
	var allFetchers []*types.AccessGraphAWSSync

	s.dynamicDiscoveryConfigMu.RLock()
	for _, discConfig := range s.dynamicDiscoveryConfig {
		if discConfig.Spec.AccessGraph == nil || len(discConfig.Spec.AccessGraph.AWS) == 0 {
			continue
		}

		for _, disc := range discConfig.Spec.AccessGraph.AWS {
			if disc.CloudTrailLogs != nil {
				allFetchers = append(allFetchers, disc)
			}
		}

	}
	s.dynamicDiscoveryConfigMu.RUnlock()

	if s.Config.Matchers.AccessGraph == nil {
		return allFetchers
	}
	for _, disc := range s.Config.Matchers.AccessGraph.AWS {
		if disc.CloudTrailLogs != nil {
			allFetchers = append(allFetchers, disc)
		}
	}
	return allFetchers
}

func pushUpsertInBatches(
	client accessgraphv1alpha.AccessGraphService_AWSEventsStreamClient,
	upsert *accessgraphv1alpha.AWSResourceList,
) error {
	for i := 0; i < len(upsert.Resources); i += batchSize {
		end := min(i+batchSize, len(upsert.Resources))
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
		end := min(i+batchSize, len(toDel.Resources))
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
	// Set the maximum message size to 50MB because we wan't to forward the raw
	// gzip compressed S3 object to the access graph service.
	// AWS splits the files uncompressed into 50MB chunks, so we need to be able
	// to send the whole file in one go. Usually the files are smaller than
	// 10MB compressed, but we want to be able to send the whole file in one go.
	const maxMessageSize = 50 * 1024 * 1024 // 50MB
	opts = append(opts,
		opt,
		grpc.WithUnaryInterceptor(metadata.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(metadata.StreamClientInterceptor),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxMessageSize),
			grpc.MaxCallSendMsgSize(maxMessageSize),
		),
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
			reloadCh := s.newDiscoveryConfigChangedSub()
			for {
				allFetchers := s.getAllAWSSyncFetchersWithTrailEnabled()
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
				if err := s.startCloudtrailPoller(ctx, reloadCh, allFetchers); errors.Is(err, errTAGFeatureNotEnabled) {
					s.Log.WarnContext(ctx, "Access Graph specified in config, but the license does not include Teleport Identity Security. Access graph sync will not be enabled.")
					break
				} else if err != nil {
					s.Log.WarnContext(ctx, "Error initializing and watching access graph", "error", err)
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Minute):
				case <-reloadCh:
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

func (s *Server) startCloudtrailPoller(ctx context.Context, reloadCh <-chan struct{}, matchers []*types.AccessGraphAWSSync) error {
	// aws discovery semaphore lock.
	const semaphoreName = "access_graph_aws_cloudtrail_sync"

	clusterFeatures := s.Config.ClusterFeatures()
	policy := modules.GetProtoEntitlement(&clusterFeatures, entitlements.Policy)
	if !clusterFeatures.AccessGraph && !policy.Enabled {
		return trace.Wrap(errTAGFeatureNotEnabled)
	}

	const semaphoreExpiration = time.Minute
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
				Config: &accessgraphv1alpha.AWSCloudTrailConfig{},
			},
		},
	)
	if err != nil {
		err = consumeTillErr(stream)
		s.Log.ErrorContext(ctx, "Failed to send access graph config", "error", err)
		return trace.Wrap(err)
	}

	if err := s.receiveTAGConfigFromStream(ctx, stream); err != nil {
		return trace.Wrap(err, "failed to receive access graph config")
	}

	if err := s.receiveTAGResumeFromStream(ctx, stream); err != nil {
		return trace.Wrap(err, "failed to receive access graph resume")
	}

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

	filePayload := make(chan payloadChannelMessage, 1)
	type mapPayload struct {
		disc       *types.AccessGraphAWSSync
		cancelFunc context.CancelFunc
	}
	stateMatchersMap := make(map[string]mapPayload)

	spawnMatcher := func(ctx context.Context, matcher *types.AccessGraphAWSSync) {
		localCtx, cancel := context.WithCancel(ctx)
		stateMatchersMap[matcher.Integration] = mapPayload{
			disc:       matcher,
			cancelFunc: cancel,
		}
		go func(ctx context.Context, matcher *types.AccessGraphAWSSync) {
			accountID, err := s.getAccountId(ctx, matcher)
			if err != nil {
				s.Log.ErrorContext(ctx, "Error getting account ID", "error", err)
				return
			}
			err = s.pollEventsFromSQSFiles(ctx, accountID, matcher, filePayload)
			if err != nil {
				s.Log.ErrorContext(ctx, "Error extracting events from SQS files", "error", err)
			}
		}(localCtx, matcher)
	}

	for _, matcher := range matchers {
		if matcher.CloudTrailLogs == nil {
			continue
		}
		spawnMatcher(ctx, matcher)
	}

	reconciler, err := services.NewGenericReconciler(services.GenericReconcilerConfig[string, *types.AccessGraphAWSSync]{
		Matcher: func(matcher *types.AccessGraphAWSSync) bool {
			return true
		},
		GetCurrentResources: func() map[string]*types.AccessGraphAWSSync {
			matchersMap := make(map[string]*types.AccessGraphAWSSync)
			for k, matcher := range stateMatchersMap {
				matchersMap[k] = matcher.disc
			}
			return matchersMap
		},
		GetNewResources: func() map[string]*types.AccessGraphAWSSync {
			matchersMap := make(map[string]*types.AccessGraphAWSSync)
			for _, matcher := range s.getAllAWSSyncFetchersWithTrailEnabled() {
				matchersMap[matcher.Integration] = matcher
			}
			return matchersMap
		},
		// Compare allows custom comparators without having to implement IsEqual.
		// Defaults to `CompareResources[T]` if not specified.
		CompareResources: services.CompareResources[*types.AccessGraphAWSSync],
		OnCreate: func(_ context.Context, disc *types.AccessGraphAWSSync) error {
			spawnMatcher(ctx, disc)
			return nil
		},
		// OnUpdate is called when an existing resource is updated.
		OnUpdate: func(ctx context.Context, new, old *types.AccessGraphAWSSync) error {
			if p, ok := stateMatchersMap[old.Integration]; ok {
				p.cancelFunc()
				delete(stateMatchersMap, old.Integration)
			}
			spawnMatcher(ctx, new)
			return nil
		},
		// OnDelete is called when an existing resource is deleted.
		OnDelete: func(_ context.Context, disc *types.AccessGraphAWSSync) error {
			if p, ok := stateMatchersMap[disc.Integration]; ok {
				p.cancelFunc()
				delete(stateMatchersMap, disc.Integration)
			}
			return nil
		},
		Logger: s.Log,
	})
	if err != nil {
		s.Log.ErrorContext(ctx, "Error creating reconciler", "error", err)
		return trace.Wrap(err)
	}
	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-reloadCh:
			if err := reconciler.Reconcile(ctx); err != nil {
				s.Log.ErrorContext(ctx, "Error reconciling access graph fetchers", "error", err)
			}
			continue
		case file := <-filePayload:
			err := stream.Send(
				&accessgraphv1alpha.AWSCloudTrailStreamRequest{
					Action: &accessgraphv1alpha.AWSCloudTrailStreamRequest_EventsFile{
						EventsFile: &accessgraphv1alpha.AWSCloudTrailEventsFile{
							Payload:      file.payload,
							AwsAccountId: file.accountID,
						},
					},
				},
			)
			if err != nil {
				err = consumeTillErr(stream)
				s.Log.ErrorContext(ctx, "Failed to send access graph service events", "error", err)
				return trace.Wrap(err)
			}
			continue
		}
	}
}

// receiveTAGConfigFromStream receives the TAG config from the stream.
func (s *Server) receiveTAGConfigFromStream(ctx context.Context, stream accessgraphv1alpha.AccessGraphService_AWSCloudTrailStreamClient) error {
	tagAWSConfig, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, " failed to receive config")
	}

	if tagAWSConfig.GetCloudTrailConfig() == nil {
		return trace.BadParameter("access graph service did not return cloud trail config")
	}

	s.Log.InfoContext(ctx, "Access graph service cloud trail config", "config", tagAWSConfig.GetCloudTrailConfig())
	return nil
}

// receiveTAGConfigFromStream receives the TAG config from the stream.
func (s *Server) receiveTAGResumeFromStream(ctx context.Context, stream accessgraphv1alpha.AccessGraphService_AWSCloudTrailStreamClient) error {
	tagAWSResume, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, " failed to receive resume state")
	}

	if tagAWSResume.GetResumeState() == nil {
		return trace.BadParameter("access graph service did not return resume state")
	}

	s.Log.InfoContext(ctx, "Access graph service resume state", "resume_state", tagAWSResume.GetResumeState())
	return nil
}

func consumeTillErr(stream accessgraphv1alpha.AccessGraphService_AWSCloudTrailStreamClient) error {
	for {
		_, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func getOptions(matcher *types.AccessGraphAWSSync) []awsconfig.OptionsFn {
	opts := []awsconfig.OptionsFn{
		awsconfig.WithCredentialsMaybeIntegration(matcher.Integration),
	}
	if matcher.AssumeRole != nil {
		opts = append(opts, awsconfig.WithAssumeRole(matcher.AssumeRole.RoleARN, matcher.AssumeRole.ExternalID))
	}
	return opts
}

func (s *Server) getAccountId(ctx context.Context, matcher *types.AccessGraphAWSSync) (string, error) {
	awsCfg, err := s.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because groups are global */
		getOptions(matcher)...,
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

type payloadChannelMessage struct {
	payload   []byte
	accountID string
}

func (s *Server) pollEventsFromSQSFiles(ctx context.Context, accountID string, matcher *types.AccessGraphAWSSync, eventsC chan<- payloadChannelMessage) error {
	awsCfg, err := s.AWSConfigProvider.GetConfig(ctx, matcher.CloudTrailLogs.Region, getOptions(matcher)...)
	if err != nil {
		return trace.Wrap(err)
	}
	sqsClient := sqs.NewFromConfig(awsCfg)
	s3Client := s3.NewFromConfig(awsCfg)

	return s.pollEventsFromSQSFilesImpl(ctx, accountID, sqsClient, s3Client, matcher.CloudTrailLogs, eventsC)
}

type sqsClient interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}
type s3Client interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

func (s *Server) pollEventsFromSQSFilesImpl(ctx context.Context,
	accountID string,
	sqsClient sqsClient,
	s3Client s3Client,
	matcher *types.AccessGraphAWSSyncCloudTrailLogs,
	eventsC chan<- payloadChannelMessage,
) error {
	parallelDownloads := make(chan struct{}, 60)
	errG, ctx := errgroup.WithContext(ctx)
	for range 10 {
		errG.Go(
			s.processMessagesWorker(
				ctx,
				matcher,
				sqsClient,
				s3Client,
				eventsC,
				accountID,
				parallelDownloads,
			),
		)
	}
	return errG.Wait()
}

func (s *Server) processMessagesWorker(
	ctx context.Context,
	matcher *types.AccessGraphAWSSyncCloudTrailLogs,
	sqsClient sqsClient,
	s3Client s3Client,
	eventsC chan<- payloadChannelMessage,
	accountID string,
	parallelDownloads chan struct{},
) func() error {
	return func() error {
		input := &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(matcher.SQSQueue),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     10,
		}
		for {
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			default:
				resp, err := sqsClient.ReceiveMessage(ctx, input)
				if err != nil {
					s.Log.ErrorContext(ctx, "Failed to receive message from SQS", "error", err)
					continue
				}
				if len(resp.Messages) == 0 {
					continue
				}
				for _, message := range resp.Messages {
					parallelDownloads <- struct{}{}
					go func(message sqstypes.Message) {
						defer func() {
							<-parallelDownloads
						}()
						if err := s.processSingleMessage(ctx, message, s3Client, eventsC, accountID); err != nil {
							s.Log.ErrorContext(ctx, "Failed to process SQS message", "error", err, "message_payload", aws.ToString(message.Body), "message_id", aws.ToString(message.MessageId))
							return
						}
						if message.ReceiptHandle != nil {
							_, err := sqsClient.DeleteMessage(
								ctx,
								&sqs.DeleteMessageInput{
									QueueUrl:      aws.String(matcher.SQSQueue),
									ReceiptHandle: message.ReceiptHandle,
								})
							if err != nil {
								s.Log.WarnContext(ctx, "Failed to delete message from sqs", "error", err, "message_id", aws.ToString(message.MessageId))
							}
						} else {
							s.Log.ErrorContext(ctx, "Skipping message deletion as ReceiptHandle is nil", "message_id", aws.ToString(message.MessageId))
						}
					}(message)
				}

			}
		}
	}
}

func (s *Server) processSingleMessage(
	ctx context.Context,
	msg sqstypes.Message,
	s3Client s3Client,
	eventsC chan<- payloadChannelMessage,
	accountID string,
) error {
	var sqsEvent sqsFileEvent
	body := aws.ToString(msg.Body)

	if err := json.Unmarshal([]byte(body), &sqsEvent); err != nil {
		s.Log.ErrorContext(ctx, "Failed to unmarshal SQS message", "error", err, "message_payload", body, "message_id", aws.ToString(msg.MessageId))
		return nil
	}

	if len(sqsEvent.S3ObjectKey) == 0 {
		s.Log.ErrorContext(ctx, "SQS message does not contain S3 object key", "message_payload", body, "message_id", aws.ToString(msg.MessageId))
		return nil
	}

	var payloads [][]byte
	for _, objectKey := range sqsEvent.S3ObjectKey {
		if len(objectKey) == 0 {
			s.Log.ErrorContext(ctx, "SQS message contains empty S3 object key", "message_payload", body, "message_id", aws.ToString(msg.MessageId))
			continue
		}

		// We don't need to retry the download if it fails
		// because the SQS message will be requeued after.
		payload, err := downloadCloudTrailFile(ctx, s3Client, sqsEvent.S3Bucket, objectKey)
		if err != nil {
			s.Log.ErrorContext(ctx, "Failed to download and parse S3 object", "error", err, "message_payload", body, "nessage_id", aws.ToString(msg.MessageId))
			return trace.Wrap(err)
		}

		// capture the payload to send when all downloads are done.
		payloads = append(payloads, payload)
	}

	for _, payload := range payloads {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case eventsC <- payloadChannelMessage{
			payload:   payload,
			accountID: accountID,
		}:

		}
	}
	return nil
}

type sqsFileEvent struct {
	S3Bucket    string   `json:"s3Bucket"`
	S3ObjectKey []string `json:"s3ObjectKey"`
}

// downloadCloudTrailFile downloads the S3 object with the given bucket and key
// and returns its contents as a byte slice.
func downloadCloudTrailFile(ctx context.Context, client s3Client, bucket, key string) (_ []byte, returnErr error) {
	getObjInput := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	resp, err := client.GetObject(ctx, getObjInput)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get S3 object")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			returnErr = trace.NewAggregate(returnErr, err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read S3 object body")
	}

	return body, nil
}
