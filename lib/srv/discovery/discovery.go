/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
	"github.com/gravitational/teleport/lib/srv/discovery/fetchers/db"
	"github.com/gravitational/teleport/lib/srv/server"
	"github.com/gravitational/teleport/lib/utils/spreadwork"
)

var errNoInstances = errors.New("all fetched nodes already enrolled")

// Matchers contains all matchers used by discovery service
type Matchers struct {
	// AWS is a list of AWS EC2 matchers.
	AWS []types.AWSMatcher
	// Azure is a list of Azure matchers to discover resources.
	Azure []types.AzureMatcher
	// GCP is a list of GCP matchers to discover resources.
	GCP []types.GCPMatcher
	// Kubernetes is a list of Kubernetes matchers to discovery resources.
	Kubernetes []types.KubernetesMatcher
	// AccessGraph is the configuration for the Access Graph Cloud sync.
	AccessGraph *types.AccessGraphSync
}

func (m Matchers) IsEmpty() bool {
	return len(m.GCP) == 0 &&
		len(m.AWS) == 0 &&
		len(m.Azure) == 0 &&
		len(m.Kubernetes) == 0 &&
		(m.AccessGraph == nil || len(m.AccessGraph.AWS) == 0)
}

// ssmInstaller handles running SSM commands that install Teleport on EC2 instances.
type ssmInstaller interface {
	Run(ctx context.Context, req server.SSMRunRequest) error
}

// azureInstaller handles running commands that install Teleport on Azure
// virtual machines.
type azureInstaller interface {
	Run(ctx context.Context, req server.AzureRunRequest) error
}

// gcpInstaller handles running commands that install Teleport on GCP
// virtual machines.
type gcpInstaller interface {
	Run(ctx context.Context, req server.GCPRunRequest) error
}

// Config provides configuration for the discovery server.
type Config struct {
	// CloudClients is an interface for retrieving cloud clients.
	CloudClients cloud.Clients
	// IntegrationOnlyCredentials discards any Matcher that don't have an Integration.
	// When true, ambient credentials (used by the Cloud SDKs) are not used.
	IntegrationOnlyCredentials bool
	// KubernetesClient is the Kubernetes client interface
	KubernetesClient kubernetes.Interface
	// Matchers stores all types of matchers to discover resources
	Matchers Matchers
	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter
	// AccessPoint is a discovery access point
	AccessPoint auth.DiscoveryAccessPoint
	// Log is the logger.
	Log logrus.FieldLogger
	// ServerID identifies the Teleport instance where this service runs.
	ServerID string
	// onDatabaseReconcile is called after each database resource reconciliation.
	onDatabaseReconcile func()
	// protocolChecker is used by Kubernetes fetchers to check port's protocol if needed.
	protocolChecker fetchers.ProtocolChecker
	// DiscoveryGroup is the name of the discovery group that the current
	// discovery service is a part of.
	// It is used to filter out discovered resources that belong to another
	// discovery services. When running in high availability mode and the agents
	// have access to the same cloud resources, this field value must be the same
	// for all discovery services. If different agents are used to discover different
	// sets of cloud resources, this field must be different for each set of agents.
	DiscoveryGroup string
	// ClusterName is the name of the Teleport cluster.
	ClusterName string
	// PollInterval is the cadence at which the discovery server will run each of its
	// discovery cycles.
	PollInterval time.Duration

	// ServerCredentials are the credentials used to identify the discovery service
	// to the Access Graph service.
	ServerCredentials *tls.Config
	// AccessGraphConfig is the configuration for the Access Graph client
	AccessGraphConfig AccessGraphConfig

	// ClusterFeatures returns flags for supported/unsupported features.
	// Used as a function because cluster features might change on Auth restarts.
	ClusterFeatures func() proto.Features

	// TriggerFetchC is a list of channels that must be notified when a off-band poll must be performed.
	// This is used to start a polling iteration when a new DiscoveryConfig change is received.
	TriggerFetchC  []chan struct{}
	triggerFetchMu sync.RWMutex

	// clock is passed to watchers to handle poll intervals.
	// Mostly used in tests.
	clock clockwork.Clock

	// jitter is a function which applies random jitter to a duration.
	// It is used to add Expiration times to Resources that don't support Heartbeats (eg EICE Nodes).
	jitter retryutils.Jitter
}

// AccessGraphConfig represents TAG server config.
type AccessGraphConfig struct {
	// Enabled indicates if Access Graph reporting is enabled.
	Enabled bool

	// Addr of the Access Graph service.
	Addr string

	// CA is the CA in PEM format used by the Access Graph service.
	CA []byte

	// Insecure is true if the connection to the Access Graph service should be insecure.
	Insecure bool
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Matchers.IsEmpty() && c.DiscoveryGroup == "" {
		return trace.BadParameter("no matchers or discovery group configured for discovery")
	}
	if c.Emitter == nil {
		return trace.BadParameter("no Emitter configured for discovery")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("no AccessPoint configured for discovery")
	}

	if len(c.Matchers.Kubernetes) > 0 && c.DiscoveryGroup == "" {
		return trace.BadParameter(`the DiscoveryGroup name should be set for discovery server if
kubernetes matchers are present.`)
	}
	if c.CloudClients == nil {
		awsIntegrationSessionProvider := func(ctx context.Context, region, integration string) (*session.Session, error) {
			return awsoidc.NewSessionV1(ctx, c.AccessPoint, region, integration)
		}
		cloudClients, err := cloud.NewClients(
			cloud.WithAWSIntegrationSessionProvider(awsIntegrationSessionProvider),
		)
		if err != nil {
			return trace.Wrap(err, "unable to create cloud clients")
		}
		c.CloudClients = cloudClients
	}
	if c.KubernetesClient == nil && len(c.Matchers.Kubernetes) > 0 {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return trace.Wrap(err,
				"the Kubernetes App Discovery requires a Teleport Kube Agent running on a Kubernetes cluster")
		}
		kubeClient, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return trace.Wrap(err, "unable to create Kubernetes client")
		}

		c.KubernetesClient = kubeClient
	}

	if c.Log == nil {
		c.Log = logrus.New()
	}
	if c.protocolChecker == nil {
		c.protocolChecker = fetchers.NewProtoChecker(false)
	}

	if c.PollInterval == 0 {
		c.PollInterval = 5 * time.Minute
	}

	c.TriggerFetchC = make([]chan struct{}, 0)
	c.triggerFetchMu = sync.RWMutex{}

	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}

	if c.ClusterFeatures == nil {
		return trace.BadParameter("cluster features are required")
	}

	c.Log = c.Log.WithField(teleport.ComponentKey, teleport.ComponentDiscovery)

	if c.DiscoveryGroup == "" {
		c.Log.Warn("discovery_service.discovery_group is not set. This field is required for the discovery service to work properly.\n" +
			"Please set discovery_service.discovery_group according to the documentation: https://goteleport.com/docs/reference/config/#discovery-service")
	}

	c.Matchers.Azure = services.SimplifyAzureMatchers(c.Matchers.Azure)

	c.jitter = retryutils.NewSeventhJitter()

	return nil
}

// Server is a discovery server, used to discover cloud resources for
// inclusion in Teleport
type Server struct {
	*Config

	ctx context.Context
	// cancelfn is used with ctx when stopping the discovery server
	cancelfn context.CancelFunc
	// nodeWatcher is a node watcher.
	nodeWatcher *services.NodeWatcher

	// ec2Watcher periodically retrieves EC2 instances.
	ec2Watcher *server.Watcher
	// ec2Installer is used to start the installation process on discovered EC2 nodes
	ec2Installer ssmInstaller
	// azureWatcher periodically retrieves Azure virtual machines.
	azureWatcher *server.Watcher
	// azureInstaller is used to start the installation process on discovered Azure
	// virtual machines.
	azureInstaller azureInstaller
	// gcpWatcher periodically retrieves GCP virtual machines.
	gcpWatcher *server.Watcher
	// gcpInstaller is used to start the installation process on discovered GCP
	// virtual machines
	gcpInstaller gcpInstaller
	// kubeFetchers holds all non-integration based kubernetes fetchers for Azure and other clouds.
	kubeFetchers []common.Fetcher
	// kubeAppsFetchers holds all kubernetes fetchers for apps.
	kubeAppsFetchers []common.Fetcher
	// databaseFetchers holds all database fetchers.
	databaseFetchers []common.Fetcher

	// dynamicMatcherWatcher is an initialized Watcher for DiscoveryConfig resources.
	// Each new event must update the existing resources.
	dynamicMatcherWatcher types.Watcher

	// dynamicDatabaseFetchers holds the current Database Fetchers for the Dynamic Matchers (those coming from DiscoveryConfig resource).
	// The key is the DiscoveryConfig name.
	dynamicDatabaseFetchers   map[string][]common.Fetcher
	muDynamicDatabaseFetchers sync.RWMutex

	// dynamicServerAWSFetchers holds the current AWS EC2 Fetchers for the Dynamic Matchers (those coming from DiscoveryConfig resource).
	// The key is the DiscoveryConfig name.
	dynamicServerAWSFetchers   map[string][]server.Fetcher
	muDynamicServerAWSFetchers sync.RWMutex
	staticServerAWSFetchers    []server.Fetcher

	// dynamicServerAzureFetchers holds the current Azure VM Fetchers for the Dynamic Matchers (those coming from DiscoveryConfig resource).
	// The key is the DiscoveryConfig name.
	dynamicServerAzureFetchers   map[string][]server.Fetcher
	muDynamicServerAzureFetchers sync.RWMutex
	staticServerAzureFetchers    []server.Fetcher

	// dynamicServerGCPFetchers holds the current GCP VM Fetchers for the Dynamic Matchers (those coming from DiscoveryConfig resource).
	// The key is the DiscoveryConfig name.
	dynamicServerGCPFetchers   map[string][]server.Fetcher
	muDynamicServerGCPFetchers sync.RWMutex
	staticServerGCPFetchers    []server.Fetcher

	// dynamicTAGSyncFetchers holds the current TAG Fetchers for the Dynamic Matchers (those coming from DiscoveryConfig resource).
	// The key is the DiscoveryConfig name.
	dynamicTAGSyncFetchers   map[string][]aws_sync.AWSSync
	muDynamicTAGSyncFetchers sync.RWMutex
	staticTAGSyncFetchers    []aws_sync.AWSSync

	// dynamicKubeIntegrationFetchers holds the current kube fetchers that use integration as a source of credentials,
	// for the Dynamic Matchers (those coming from DiscoveryConfig resource).
	// The key is the DiscoveryConfig name.
	dynamicKubeIntegrationFetchers   map[string][]common.Fetcher
	muDynamicKubeIntegrationFetchers sync.RWMutex

	// caRotationCh receives nodes that need to have their CAs rotated.
	caRotationCh chan []types.Server
	// reconciler periodically reconciles the labels of discovered instances
	// with the auth server.
	reconciler *labelReconciler

	mu sync.Mutex
	// usageEventCache keeps track of which instances the server has emitted
	// usage events for.
	usageEventCache map[string]struct{}
}

// New initializes a discovery Server
func New(ctx context.Context, cfg *Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	localCtx, cancelfn := context.WithCancel(ctx)
	s := &Server{
		Config:                         cfg,
		ctx:                            localCtx,
		cancelfn:                       cancelfn,
		usageEventCache:                make(map[string]struct{}),
		dynamicKubeIntegrationFetchers: make(map[string][]common.Fetcher),
		dynamicDatabaseFetchers:        make(map[string][]common.Fetcher),
		dynamicServerAWSFetchers:       make(map[string][]server.Fetcher),
		dynamicServerAzureFetchers:     make(map[string][]server.Fetcher),
		dynamicServerGCPFetchers:       make(map[string][]server.Fetcher),
		dynamicTAGSyncFetchers:         make(map[string][]aws_sync.AWSSync),
	}
	s.discardUnsupportedMatchers(&s.Matchers)

	if err := s.startDynamicMatchersWatcher(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	databaseFetchers, err := s.databaseFetchersFromMatchers(cfg.Matchers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.databaseFetchers = databaseFetchers

	if err := s.initAWSWatchers(cfg.Matchers.AWS); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.initAzureWatchers(ctx, cfg.Matchers.Azure); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.initGCPWatchers(ctx, cfg.Matchers.GCP); err != nil {
		return nil, trace.Wrap(err)
	}

	if s.ec2Watcher != nil || s.azureWatcher != nil || s.gcpWatcher != nil {
		if err := s.initTeleportNodeWatcher(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := s.initKubeAppWatchers(cfg.Matchers.Kubernetes); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.initAccessGraphWatchers(ctx, cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	return s, nil
}

// startDynamicMatchersWatcher starts a watcher for DiscoveryConfig events.
// After initialization, it starts a goroutine that receives and handles events.
func (s *Server) startDynamicMatchersWatcher(ctx context.Context) error {
	if s.DiscoveryGroup == "" {
		return nil
	}

	watcher, err := s.AccessPoint.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindDiscoveryConfig,
		}},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Wait for OpInit event so the watcher is ready.
	select {
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("failed to watch for DiscoveryConfig: received an unexpected event while waiting for the initial OpInit")
		}
	case <-watcher.Done():
		return trace.Wrap(watcher.Error())
	}

	s.dynamicMatcherWatcher = watcher

	if err := s.loadExistingDynamicDiscoveryConfigs(); err != nil {
		return trace.Wrap(err)
	}

	go s.startDynamicWatcherUpdater()
	return nil
}

// initAWSWatchers starts AWS resource watchers based on types provided.
func (s *Server) initAWSWatchers(matchers []types.AWSMatcher) error {
	var err error

	ec2Matchers, otherMatchers := splitMatchers(matchers, func(matcherType string) bool {
		return matcherType == types.AWSMatcherEC2
	})

	s.staticServerAWSFetchers, err = server.MatchersToEC2InstanceFetchers(s.ctx, ec2Matchers, s.CloudClients)
	if err != nil {
		return trace.Wrap(err)
	}

	s.ec2Watcher, err = server.NewEC2Watcher(
		s.ctx, s.getAllAWSServerFetchers, s.caRotationCh,
		server.WithPollInterval(s.PollInterval),
		server.WithTriggerFetchC(s.newDiscoveryConfigChangedSub()),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	s.caRotationCh = make(chan []types.Server)

	if s.ec2Installer == nil {
		s.ec2Installer = server.NewSSMInstaller(server.SSMInstallerConfig{
			Emitter: s.Emitter,
		})
	}

	lr, err := newLabelReconciler(&labelReconcilerConfig{
		log:         s.Log,
		accessPoint: s.AccessPoint,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	s.reconciler = lr

	// Database fetchers were added in databaseFetchersFromMatchers.
	_, otherMatchers = splitMatchers(otherMatchers, db.IsAWSMatcherType)

	// Add non-integration kube fetchers.
	kubeFetchers, _, err := fetchers.MakeEKSFetchersFromAWSMatchers(s.Log, s.CloudClients, otherMatchers)
	if err != nil {
		return trace.Wrap(err)
	}
	s.kubeFetchers = append(s.kubeFetchers, kubeFetchers...)

	return nil
}

func (s *Server) initKubeAppWatchers(matchers []types.KubernetesMatcher) error {
	if len(matchers) == 0 {
		return nil
	}

	kubeClient := s.KubernetesClient
	if kubeClient == nil {
		return trace.BadParameter("Kubernetes client is not present")
	}

	for _, matcher := range matchers {
		if !slices.Contains(matcher.Types, types.KubernetesMatchersApp) {
			continue
		}

		fetcher, err := fetchers.NewKubeAppsFetcher(fetchers.KubeAppsFetcherConfig{
			KubernetesClient: kubeClient,
			FilterLabels:     matcher.Labels,
			Namespaces:       matcher.Namespaces,
			Log:              s.Log,
			ClusterName:      s.DiscoveryGroup,
			ProtocolChecker:  s.Config.protocolChecker,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		s.kubeAppsFetchers = append(s.kubeAppsFetchers, fetcher)
	}
	return nil
}

// awsServerFetchersFromMatchers converts Matchers into a set of AWS EC2 Fetchers.
func (s *Server) awsServerFetchersFromMatchers(ctx context.Context, matchers []types.AWSMatcher) ([]server.Fetcher, error) {
	serverMatchers, _ := splitMatchers(matchers, func(matcherType string) bool {
		return matcherType == types.AWSMatcherEC2
	})

	fetchers, err := server.MatchersToEC2InstanceFetchers(ctx, serverMatchers, s.CloudClients)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return fetchers, nil
}

// azureServerFetchersFromMatchers converts Matchers into a set of Azure Servers Fetchers.
func (s *Server) azureServerFetchersFromMatchers(matchers []types.AzureMatcher) []server.Fetcher {
	serverMatchers, _ := splitMatchers(matchers, func(matcherType string) bool {
		return matcherType == types.AzureMatcherVM
	})

	return server.MatchersToAzureInstanceFetchers(serverMatchers, s.CloudClients)
}

// gcpServerFetchersFromMatchers converts Matchers into a set of GCP Servers Fetchers.
func (s *Server) gcpServerFetchersFromMatchers(ctx context.Context, matchers []types.GCPMatcher) ([]server.Fetcher, error) {
	serverMatchers, _ := splitMatchers(matchers, func(matcherType string) bool {
		return matcherType == types.GCPMatcherCompute
	})

	if len(serverMatchers) == 0 {
		// We have an early exit here because GetGCPInstancesClient returns an error
		// when there are no credentials in the environment.
		return nil, nil
	}

	client, err := s.CloudClients.GetGCPInstancesClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return server.MatchersToGCPInstanceFetchers(serverMatchers, client), nil
}

// databaseFetchersFromMatchers converts Matchers into a set of Database Fetchers.
func (s *Server) databaseFetchersFromMatchers(matchers Matchers) ([]common.Fetcher, error) {
	var fetchers []common.Fetcher

	// AWS
	awsDatabaseMatchers, _ := splitMatchers(matchers.AWS, db.IsAWSMatcherType)
	if len(awsDatabaseMatchers) > 0 {
		databaseFetchers, err := db.MakeAWSFetchers(s.ctx, s.CloudClients, awsDatabaseMatchers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fetchers = append(fetchers, databaseFetchers...)
	}

	// Azure
	azureDatabaseMatchers, _ := splitMatchers(matchers.Azure, db.IsAzureMatcherType)
	if len(azureDatabaseMatchers) > 0 {
		databaseFetchers, err := db.MakeAzureFetchers(s.CloudClients, azureDatabaseMatchers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fetchers = append(fetchers, databaseFetchers...)
	}

	// There are no Database Matchers for GCP Matchers.
	// There are no Database Matchers for Kube Matchers.

	return fetchers, nil
}

func (s *Server) kubeIntegrationFetchersFromMatchers(matchers Matchers) ([]common.Fetcher, error) {
	var result []common.Fetcher

	// AWS
	awsKubeMatchers, _ := splitMatchers(matchers.AWS, func(matcherType string) bool {
		return matcherType == types.AWSMatcherEKS
	})
	if len(awsKubeMatchers) > 0 {
		_, kubeIntegrationFetchers, err := fetchers.MakeEKSFetchersFromAWSMatchers(s.Log, s.CloudClients, awsKubeMatchers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, kubeIntegrationFetchers...)
	}

	// There can't be kube integration fetchers for other matcher types.

	return result, nil
}

// initAzureWatchers starts Azure resource watchers based on types provided.
func (s *Server) initAzureWatchers(ctx context.Context, matchers []types.AzureMatcher) error {
	vmMatchers, otherMatchers := splitMatchers(matchers, func(matcherType string) bool {
		return matcherType == types.AzureMatcherVM
	})

	s.staticServerAzureFetchers = server.MatchersToAzureInstanceFetchers(vmMatchers, s.CloudClients)

	// VM watcher.
	var err error
	s.azureWatcher, err = server.NewAzureWatcher(
		s.ctx, s.getAllAzureServerFetchers,
		server.WithPollInterval(s.PollInterval),
		server.WithTriggerFetchC(s.newDiscoveryConfigChangedSub()),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	if s.azureInstaller == nil {
		s.azureInstaller = &server.AzureInstaller{
			Emitter: s.Emitter,
		}
	}

	// Database fetchers were added in databaseFetchersFromMatchers.
	_, otherMatchers = splitMatchers(otherMatchers, db.IsAzureMatcherType)

	// Add kube fetchers.
	for _, matcher := range otherMatchers {
		subscriptions, err := s.getAzureSubscriptions(ctx, matcher.Subscriptions)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, subscription := range subscriptions {
			for _, t := range matcher.Types {
				switch t {
				case types.AzureMatcherKubernetes:
					kubeClient, err := s.CloudClients.GetAzureKubernetesClient(subscription)
					if err != nil {
						return trace.Wrap(err)
					}
					fetcher, err := fetchers.NewAKSFetcher(fetchers.AKSFetcherConfig{
						Client:         kubeClient,
						Regions:        matcher.Regions,
						FilterLabels:   matcher.ResourceTags,
						ResourceGroups: matcher.ResourceGroups,
						Log:            s.Log,
					})
					if err != nil {
						return trace.Wrap(err)
					}
					s.kubeFetchers = append(s.kubeFetchers, fetcher)
				}
			}
		}
	}
	return nil
}

func (s *Server) initGCPServerWatcher(ctx context.Context, vmMatchers []types.GCPMatcher) error {
	var err error

	s.staticServerGCPFetchers, err = s.gcpServerFetchersFromMatchers(ctx, vmMatchers)
	if err != nil {
		return trace.Wrap(err)
	}

	s.gcpWatcher, err = server.NewGCPWatcher(
		s.ctx, s.getAllGCPServerFetchers,
		server.WithPollInterval(s.PollInterval),
		server.WithTriggerFetchC(s.newDiscoveryConfigChangedSub()),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if s.gcpInstaller == nil {
		s.gcpInstaller = &server.GCPInstaller{
			Emitter: s.Emitter,
		}
	}

	return nil
}

// initGCPWatchers starts GCP resource watchers based on types provided.
func (s *Server) initGCPWatchers(ctx context.Context, matchers []types.GCPMatcher) error {
	// return early if there are no matchers as GetGCPGKEClient causes
	// an error if there are no credentials present

	vmMatchers, otherMatchers := splitMatchers(matchers, func(matcherType string) bool {
		return matcherType == types.GCPMatcherCompute
	})

	if err := s.initGCPServerWatcher(ctx, vmMatchers); err != nil {
		return trace.Wrap(err)
	}

	// If there's no GCP Client creds in the environment
	// and call to GetGCP...Client
	// Early exit if there's no Kube Matchers, to prevent the error.
	if len(otherMatchers) == 0 {
		return nil
	}

	kubeClient, err := s.CloudClients.GetGCPGKEClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, matcher := range otherMatchers {
		for _, projectID := range matcher.ProjectIDs {
			for _, location := range matcher.Locations {
				for _, t := range matcher.Types {
					switch t {
					case types.GCPMatcherKubernetes:
						fetcher, err := fetchers.NewGKEFetcher(fetchers.GKEFetcherConfig{
							Client:       kubeClient,
							Location:     location,
							FilterLabels: matcher.GetLabels(),
							ProjectID:    projectID,
							Log:          s.Log,
						})
						if err != nil {
							return trace.Wrap(err)
						}
						s.kubeFetchers = append(s.kubeFetchers, fetcher)
					}
				}
			}
		}
	}
	return nil
}

func (s *Server) filterExistingEC2Nodes(instances *server.EC2Instances) {
	nodes := s.nodeWatcher.GetNodes(s.ctx, func(n services.Node) bool {
		labels := n.GetAllLabels()
		_, accountOK := labels[types.AWSAccountIDLabel]
		_, instanceOK := labels[types.AWSInstanceIDLabel]
		return accountOK && instanceOK
	})

	var filtered []server.EC2Instance
outer:
	for _, inst := range instances.Instances {
		for _, node := range nodes {
			match := types.MatchLabels(node, map[string]string{
				types.AWSAccountIDLabel:  instances.AccountID,
				types.AWSInstanceIDLabel: inst.InstanceID,
			})
			if match {
				continue outer
			}
		}
		filtered = append(filtered, inst)
	}
	instances.Instances = filtered
}

func genEC2InstancesLogStr(instances []server.EC2Instance) string {
	return genInstancesLogStr(instances, func(i server.EC2Instance) string {
		return i.InstanceID
	})
}

func genAzureInstancesLogStr(instances []*armcompute.VirtualMachine) string {
	return genInstancesLogStr(instances, func(i *armcompute.VirtualMachine) string {
		return aws.StringValue(i.Name)
	})
}

func genGCPInstancesLogStr(instances []*gcp.Instance) string {
	return genInstancesLogStr(instances, func(i *gcp.Instance) string {
		return i.Name
	})
}

func genInstancesLogStr[T any](instances []T, getID func(T) string) string {
	var logInstances strings.Builder
	for idx, inst := range instances {
		if idx == 10 || idx == (len(instances)-1) {
			logInstances.WriteString(getID(inst))
			break
		}
		logInstances.WriteString(getID(inst) + ", ")
	}
	if len(instances) > 10 {
		logInstances.WriteString(fmt.Sprintf("... + %d instance IDs truncated", len(instances)-10))
	}

	return fmt.Sprintf("[%s]", logInstances.String())
}

func (s *Server) handleEC2Instances(instances *server.EC2Instances) error {
	// TODO(marco): support AWS SSM Client backed by an integration
	serverInfos, err := instances.ServerInfos()
	if err != nil {
		return trace.Wrap(err)
	}
	s.reconciler.queueServerInfos(serverInfos)

	// instances.Rotation is true whenever the instances received need
	// to be rotated, we don't want to filter out existing OpenSSH nodes as
	// they all need to have the command run on them
	//
	// Integration/EICE Nodes don't have heartbeat.
	// Those Nodes must not be filtered, so that we can extend their expiration and sync labels.
	if !instances.Rotation && instances.Integration == "" {
		s.filterExistingEC2Nodes(instances)
	}
	if len(instances.Instances) == 0 {
		return trace.NotFound("all fetched nodes already enrolled")
	}

	switch {
	case instances.Integration != "":
		s.heartbeatEICEInstance(instances)
	default:
		if err := s.handleEC2RemoteInstallation(instances); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := s.emitUsageEvents(instances.MakeEvents()); err != nil {
		s.Log.WithError(err).Debug("Error emitting usage event.")
	}

	return nil
}

// heartbeatEICEInstance heartbeats the list of EC2 instances as Teleport (EICE) Nodes.
func (s *Server) heartbeatEICEInstance(instances *server.EC2Instances) {
	awsInfo := &types.AWSInfo{
		AccountID:   instances.AccountID,
		Region:      instances.Region,
		Integration: instances.Integration,
	}

	existingEICEServersList := s.nodeWatcher.GetNodes(s.ctx, func(n services.Node) bool {
		return n.IsEICE()
	})

	existingEICEServersMap := make(map[string]types.Server, len(existingEICEServersList))
	for _, eiceServer := range existingEICEServersList {
		si, err := types.ServerInfoForServer(eiceServer)
		if err != nil {
			s.Log.Debugf("Failed to convert Server %q to ServerInfo: %v", eiceServer.GetName(), err)
			continue
		}

		existingEICEServersMap[si.GetName()] = eiceServer
	}

	nodesToUpsert := make([]types.Server, 0, len(instances.Instances))
	// Add EC2 Instances using EICE method
	for _, ec2Instance := range instances.Instances {
		eiceNode, err := services.NewAWSNodeFromEC2v1Instance(ec2Instance.OriginalInstance, awsInfo)
		if err != nil {
			s.Log.WithField("instance_id", ec2Instance.InstanceID).Warnf("Error converting to Teleport EICE Node: %v", err)
			continue
		}

		serverInfo, err := types.ServerInfoForServer(eiceNode)
		if err != nil {
			s.Log.WithField("instance_id", ec2Instance.InstanceID).Warnf("Error converting to Teleport ServerInfo: %v", err)
			continue
		}

		if existingNode, found := existingEICEServersMap[serverInfo.GetName()]; found {
			// Re-use the same Name so that `UpsertNode` replaces the node.
			eiceNode.SetName(existingNode.GetName())

			// To reduce load, nodes are skipped if
			// - they didn't change and
			// - their expiration is far away in the future (at least 2 Poll iterations before the Node expires)
			//
			// As an example, and using the default PollInterval (5 minutes),
			// nodes that didn't change and have their expiration greater than now+15m will be skipped.
			// This gives at least another two iterations of the DiscoveryService before the node is actually removed.
			// Note: heartbeats set an expiration of 90 minutes.
			if existingNode.Expiry().After(s.clock.Now().Add(3*s.PollInterval)) &&
				services.CompareServers(existingNode, eiceNode) == services.OnlyTimestampsDifferent {
				continue
			}
		}

		eiceNodeExpiration := s.clock.Now().Add(s.jitter(serverExpirationDuration))
		eiceNode.SetExpiry(eiceNodeExpiration)
		nodesToUpsert = append(nodesToUpsert, eiceNode)
	}

	applyOverTimeConfig := spreadwork.ApplyOverTimeConfig{
		MaxDuration: s.PollInterval,
	}
	err := spreadwork.ApplyOverTime(s.ctx, applyOverTimeConfig, nodesToUpsert, func(eiceNode types.Server) {
		if _, err := s.AccessPoint.UpsertNode(s.ctx, eiceNode); err != nil {
			instanceID := eiceNode.GetAWSInstanceID()
			s.Log.WithField("instance_id", instanceID).Warnf("Error upserting EC2 instance: %v", err)
		}
	})
	if err != nil {
		s.Log.Warnf("Failed to upsert EC2 nodes: %v", err)
	}
}

func (s *Server) handleEC2RemoteInstallation(instances *server.EC2Instances) error {
	// TODO(gavin): support assume_role_arn for ec2.
	ec2Client, err := s.CloudClients.GetAWSSSMClient(s.ctx, instances.Region, cloud.WithAmbientCredentials())
	if err != nil {
		return trace.Wrap(err)
	}

	s.Log.Debugf("Running Teleport installation on these instances: AccountID: %s, Instances: %s",
		instances.AccountID, genEC2InstancesLogStr(instances.Instances))

	req := server.SSMRunRequest{
		DocumentName: instances.DocumentName,
		SSM:          ec2Client,
		Instances:    instances.Instances,
		Params:       instances.Parameters,
		Region:       instances.Region,
		AccountID:    instances.AccountID,
	}
	if err := s.ec2Installer.Run(s.ctx, req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) logHandleInstancesErr(err error) {
	var aErr awserr.Error
	if errors.As(err, &aErr) && aErr.Code() == ssm.ErrCodeInvalidInstanceId {
		s.Log.WithError(err).Error("SSM SendCommand failed with ErrCodeInvalidInstanceId. Make sure that the instances have AmazonSSMManagedInstanceCore policy assigned. Also check that SSM agent is running and registered with the SSM endpoint on that instance and try restarting or reinstalling it in case of issues. See https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_SendCommand.html#API_SendCommand_Errors for more details.")
	} else if trace.IsNotFound(err) {
		s.Log.Debug("All discovered EC2 instances are already part of the cluster.")
	} else {
		s.Log.WithError(err).Error("Failed to enroll discovered EC2 instances.")
	}
}

func (s *Server) watchCARotation(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			nodes, err := s.findUnrotatedEC2Nodes(ctx)
			if err != nil {
				if trace.IsNotFound(err) {
					s.Log.Debug("No OpenSSH nodes require CA rotation")
					continue
				}
				s.Log.Errorf("Error finding OpenSSH nodes requiring CA rotation: %s", err)
				continue
			}
			s.Log.Debugf("Found %d nodes requiring rotation", len(nodes))
			s.caRotationCh <- nodes
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) getMostRecentRotationForCAs(ctx context.Context, caTypes ...types.CertAuthType) (time.Time, error) {
	var mostRecentUpdate time.Time
	for _, caType := range caTypes {
		ca, err := s.AccessPoint.GetCertAuthority(ctx, types.CertAuthID{
			Type:       caType,
			DomainName: s.ClusterName,
		}, false)
		if err != nil {
			return time.Time{}, trace.Wrap(err)
		}
		caRot := ca.GetRotation()
		if caRot.State == types.RotationStateInProgress && caRot.Started.After(mostRecentUpdate) {
			mostRecentUpdate = caRot.Started
		}

		if caRot.LastRotated.After(mostRecentUpdate) {
			mostRecentUpdate = caRot.LastRotated
		}
	}
	return mostRecentUpdate, nil
}

func (s *Server) findUnrotatedEC2Nodes(ctx context.Context) ([]types.Server, error) {
	mostRecentCertRotation, err := s.getMostRecentRotationForCAs(ctx, types.OpenSSHCA, types.HostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	found := s.nodeWatcher.GetNodes(ctx, func(n services.Node) bool {
		if n.GetSubKind() != types.SubKindOpenSSHNode {
			return false
		}
		if _, ok := n.GetLabel(types.AWSAccountIDLabel); !ok {
			return false
		}
		if _, ok := n.GetLabel(types.AWSInstanceIDLabel); !ok {
			return false
		}

		return mostRecentCertRotation.After(n.GetRotation().LastRotated)
	})

	if len(found) == 0 {
		return nil, trace.NotFound("no unrotated nodes found")
	}
	return found, nil
}

func (s *Server) handleEC2Discovery() {
	if err := s.nodeWatcher.WaitInitialization(); err != nil {
		s.Log.WithError(err).Error("Failed to initialize nodeWatcher.")
		return
	}

	go s.ec2Watcher.Run()
	go s.watchCARotation(s.ctx)

	for {
		select {
		case instances := <-s.ec2Watcher.InstancesC:
			ec2Instances := instances.EC2
			s.Log.Debugf("EC2 instances discovered (AccountID: %s, Instances: %v), starting installation",
				ec2Instances.AccountID, genEC2InstancesLogStr(ec2Instances.Instances))

			if err := s.handleEC2Instances(ec2Instances); err != nil {
				s.logHandleInstancesErr(err)
			}
		case <-s.ctx.Done():
			s.ec2Watcher.Stop()
			return
		}
	}
}

func (s *Server) filterExistingAzureNodes(instances *server.AzureInstances) {
	nodes := s.nodeWatcher.GetNodes(s.ctx, func(n services.Node) bool {
		labels := n.GetAllLabels()
		_, subscriptionOK := labels[types.SubscriptionIDLabel]
		_, vmOK := labels[types.VMIDLabel]
		return subscriptionOK && vmOK
	})
	var filtered []*armcompute.VirtualMachine
outer:
	for _, inst := range instances.Instances {
		for _, node := range nodes {
			var vmID string
			if inst.Properties != nil {
				vmID = aws.StringValue(inst.Properties.VMID)
			}
			match := types.MatchLabels(node, map[string]string{
				types.SubscriptionIDLabel: instances.SubscriptionID,
				types.VMIDLabel:           vmID,
			})
			if match {
				continue outer
			}
		}
		filtered = append(filtered, inst)
	}
	instances.Instances = filtered
}

func (s *Server) handleAzureInstances(instances *server.AzureInstances) error {
	client, err := s.CloudClients.GetAzureRunCommandClient(instances.SubscriptionID)
	if err != nil {
		return trace.Wrap(err)
	}
	s.filterExistingAzureNodes(instances)
	if len(instances.Instances) == 0 {
		return trace.Wrap(errNoInstances)
	}

	s.Log.Debugf("Running Teleport installation on these virtual machines: SubscriptionID: %s, VMs: %s",
		instances.SubscriptionID, genAzureInstancesLogStr(instances.Instances),
	)
	req := server.AzureRunRequest{
		Client:          client,
		Instances:       instances.Instances,
		Region:          instances.Region,
		ResourceGroup:   instances.ResourceGroup,
		Params:          instances.Parameters,
		ScriptName:      instances.ScriptName,
		PublicProxyAddr: instances.PublicProxyAddr,
		ClientID:        instances.ClientID,
	}
	if err := s.azureInstaller.Run(s.ctx, req); err != nil {
		return trace.Wrap(err)
	}
	if err := s.emitUsageEvents(instances.MakeEvents()); err != nil {
		s.Log.WithError(err).Debug("Error emitting usage event.")
	}
	return nil
}

func (s *Server) handleAzureDiscovery() {
	if err := s.nodeWatcher.WaitInitialization(); err != nil {
		s.Log.WithError(err).Error("Failed to initialize nodeWatcher.")
		return
	}

	go s.azureWatcher.Run()
	for {
		select {
		case instances := <-s.azureWatcher.InstancesC:
			azureInstances := instances.Azure
			s.Log.Debugf("Azure instances discovered (SubscriptionID: %s, Instances: %v), starting installation",
				azureInstances.SubscriptionID, genAzureInstancesLogStr(azureInstances.Instances),
			)
			if err := s.handleAzureInstances(azureInstances); err != nil {
				if errors.Is(err, errNoInstances) {
					s.Log.Debug("All discovered Azure VMs are already part of the cluster.")
				} else {
					s.Log.WithError(err).Error("Failed to enroll discovered Azure VMs.")
				}
			}
		case <-s.ctx.Done():
			s.azureWatcher.Stop()
			return
		}
	}
}

func (s *Server) filterExistingGCPNodes(instances *server.GCPInstances) {
	nodes := s.nodeWatcher.GetNodes(s.ctx, func(n services.Node) bool {
		labels := n.GetAllLabels()
		_, projectIDOK := labels[types.ProjectIDLabel]
		_, zoneOK := labels[types.ZoneLabel]
		_, nameOK := labels[types.NameLabel]
		return projectIDOK && zoneOK && nameOK
	})
	var filtered []*gcp.Instance
outer:
	for _, inst := range instances.Instances {
		for _, node := range nodes {
			match := types.MatchLabels(node, map[string]string{
				types.ProjectIDLabel: inst.ProjectID,
				types.ZoneLabel:      inst.Zone,
				types.NameLabel:      inst.Name,
			})
			if match {
				continue outer
			}
		}
		filtered = append(filtered, inst)
	}
	instances.Instances = filtered
}

func (s *Server) handleGCPInstances(instances *server.GCPInstances) error {
	client, err := s.CloudClients.GetGCPInstancesClient(s.ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	s.filterExistingGCPNodes(instances)
	if len(instances.Instances) == 0 {
		return trace.Wrap(errNoInstances)
	}

	s.Log.Debugf("Running Teleport installation on these virtual machines: ProjectID: %s, VMs: %s",
		instances.ProjectID, genGCPInstancesLogStr(instances.Instances),
	)
	req := server.GCPRunRequest{
		Client:          client,
		Instances:       instances.Instances,
		ProjectID:       instances.ProjectID,
		Zone:            instances.Zone,
		Params:          instances.Parameters,
		ScriptName:      instances.ScriptName,
		PublicProxyAddr: instances.PublicProxyAddr,
	}
	if err := s.gcpInstaller.Run(s.ctx, req); err != nil {
		return trace.Wrap(err)
	}
	if err := s.emitUsageEvents(instances.MakeEvents()); err != nil {
		s.Log.WithError(err).Debug("Error emitting usage event.")
	}
	return nil
}

func (s *Server) handleGCPDiscovery() {
	if err := s.nodeWatcher.WaitInitialization(); err != nil {
		s.Log.WithError(err).Error("Failed to initialize nodeWatcher.")
		return
	}
	go s.gcpWatcher.Run()
	for {
		select {
		case instances := <-s.gcpWatcher.InstancesC:
			gcpInstances := instances.GCP
			s.Log.Debugf("GCP instances discovered (ProjectID: %s, Instances %v), starting installation",
				gcpInstances.ProjectID, genGCPInstancesLogStr(gcpInstances.Instances),
			)
			if err := s.handleGCPInstances(gcpInstances); err != nil {
				if errors.Is(err, errNoInstances) {
					s.Log.Debug("All discovered GCP VMs are already part of the cluster.")
				} else {
					s.Log.WithError(err).Error("Failed to enroll discovered GCP VMs.")
				}
			}
		case <-s.ctx.Done():
			s.gcpWatcher.Stop()
			return
		}
	}
}

func (s *Server) emitUsageEvents(events map[string]*usageeventsv1.ResourceCreateEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, event := range events {
		if _, exists := s.usageEventCache[name]; exists {
			continue
		}
		s.usageEventCache[name] = struct{}{}
		if err := s.AccessPoint.SubmitUsageEvent(s.ctx, &proto.SubmitUsageEventRequest{
			Event: &usageeventsv1.UsageEventOneOf{
				Event: &usageeventsv1.UsageEventOneOf_ResourceCreateEvent{
					ResourceCreateEvent: event,
				},
			},
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *Server) submitFetchersEvent(fetchers []common.Fetcher) {
	// Some Matcher Types have multiple fetchers, but we only care about the Matcher Type and not the actual Fetcher.
	// Example:
	// The `rds` Matcher Type creates two Fetchers: one for RDS and another one for Aurora
	// Those fetchers's `FetcherType` both return `rds`, so we end up with two entries for `rds`.
	// We must de-duplicate those entries before submitting the event.
	type fetcherType struct {
		cloud       string
		fetcherType string
	}
	fetcherTypes := map[fetcherType]struct{}{}
	for _, f := range fetchers {
		fetcherKey := fetcherType{cloud: f.Cloud(), fetcherType: f.FetcherType()}
		fetcherTypes[fetcherKey] = struct{}{}
	}
	for f := range fetcherTypes {
		s.submitFetchEvent(f.cloud, f.fetcherType)
	}
}

func (s *Server) submitFetchEvent(cloudProvider, resourceType string) {
	err := s.AccessPoint.SubmitUsageEvent(s.ctx, &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_DiscoveryFetchEvent{
				DiscoveryFetchEvent: &usageeventsv1.DiscoveryFetchEvent{
					CloudProvider: cloudProvider,
					ResourceType:  resourceType,
				},
			},
		},
	})
	if err != nil {
		s.Log.WithError(err).Debug("Error emitting discovery fetch event.")
	}
}

func (s *Server) getAllAWSServerFetchers() []server.Fetcher {
	allFetchers := make([]server.Fetcher, 0, len(s.staticServerAWSFetchers))

	s.muDynamicServerAWSFetchers.RLock()
	for _, fetcherSet := range s.dynamicServerAWSFetchers {
		allFetchers = append(allFetchers, fetcherSet...)
	}
	s.muDynamicServerAWSFetchers.RUnlock()

	allFetchers = append(allFetchers, s.staticServerAWSFetchers...)

	if len(allFetchers) > 0 {
		s.submitFetchEvent(types.CloudAWS, types.AWSMatcherEC2)
	}

	return allFetchers
}

func (s *Server) getAllAzureServerFetchers() []server.Fetcher {
	allFetchers := make([]server.Fetcher, 0, len(s.staticServerAzureFetchers))

	s.muDynamicServerAzureFetchers.RLock()
	for _, fetcherSet := range s.dynamicServerAzureFetchers {
		allFetchers = append(allFetchers, fetcherSet...)
	}
	s.muDynamicServerAzureFetchers.RUnlock()

	allFetchers = append(allFetchers, s.staticServerAzureFetchers...)

	if len(allFetchers) > 0 {
		s.submitFetchEvent(types.CloudAzure, types.AzureMatcherVM)
	}

	return allFetchers
}

func (s *Server) getAllGCPServerFetchers() []server.Fetcher {
	allFetchers := make([]server.Fetcher, 0, len(s.staticServerGCPFetchers))

	s.muDynamicServerGCPFetchers.RLock()
	for _, fetcherSet := range s.dynamicServerGCPFetchers {
		allFetchers = append(allFetchers, fetcherSet...)
	}
	s.muDynamicServerGCPFetchers.RUnlock()

	allFetchers = append(allFetchers, s.staticServerGCPFetchers...)

	if len(allFetchers) > 0 {
		s.submitFetchEvent(types.CloudGCP, types.GCPMatcherCompute)
	}

	return allFetchers
}

// Start starts the discovery service.
func (s *Server) Start() error {
	if s.ec2Watcher != nil {
		go s.handleEC2Discovery()
		go s.reconciler.run(s.ctx)
	}
	if s.azureWatcher != nil {
		go s.handleAzureDiscovery()
	}
	if s.gcpWatcher != nil {
		go s.handleGCPDiscovery()
	}
	if err := s.startKubeWatchers(); err != nil {
		return trace.Wrap(err)
	}
	if err := s.startKubeIntegrationWatchers(); err != nil {
		return trace.Wrap(err)
	}
	if err := s.startKubeAppsWatchers(); err != nil {
		return trace.Wrap(err)
	}
	if err := s.startDatabaseWatchers(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// loadExistingDynamicDiscoveryConfigs loads all the dynamic discovery configs for the current discovery group
// and setups their matchers.
func (s *Server) loadExistingDynamicDiscoveryConfigs() error {
	// Add all existing DiscoveryConfigs as matchers.
	nextKey := ""
	for {
		dcs, respNextKey, err := s.AccessPoint.ListDiscoveryConfigs(s.ctx, 0, nextKey)
		if err != nil {
			s.Log.WithError(err).Warnf("failed to list discovery configs")
			return trace.Wrap(err)
		}

		for _, dc := range dcs {
			if dc.GetDiscoveryGroup() != s.DiscoveryGroup {
				continue
			}
			if err := s.upsertDynamicMatchers(s.ctx, dc); err != nil {
				s.Log.WithError(err).Warnf("failed to update dynamic matchers for discovery config %q", dc.GetName())
				continue
			}
		}
		if respNextKey == "" {
			break
		}
		nextKey = respNextKey
	}
	return nil
}

// startDynamicWatcherUpdater watches for DiscoveryConfig resource change events.
// Before consuming changes, it iterates over all DiscoveryConfigs and
// For deleted resources, it deletes the matchers.
// For new/updated resources, it replaces the set of fetchers.
func (s *Server) startDynamicWatcherUpdater() {
	// Consume DiscoveryConfig events to update Matchers as they change.
	for {
		select {
		case event := <-s.dynamicMatcherWatcher.Events():
			switch event.Type {
			case types.OpPut:
				dc, ok := event.Resource.(*discoveryconfig.DiscoveryConfig)
				if !ok {
					s.Log.Warnf("dynamic matcher watcher: unexpected resource type %T", event.Resource)
					return
				}

				if dc.GetDiscoveryGroup() != s.DiscoveryGroup {
					// Let's assume there's a DiscoveryConfig DC1 has DiscoveryGroup DG1, which this process is monitoring.
					// If the user updates the DiscoveryGroup to DG2, then DC1 must be removed from the scope of this process.
					// We blindly delete it, in the worst case, this is a no-op.
					s.deleteDynamicFetchers(event.Resource.GetName())
					s.notifyDiscoveryConfigChanged()
					continue
				}

				if err := s.upsertDynamicMatchers(s.ctx, dc); err != nil {
					s.Log.WithError(err).Warnf("failed to update dynamic matchers for discovery config %q", dc.GetName())
					continue
				}
				s.notifyDiscoveryConfigChanged()

			case types.OpDelete:
				s.deleteDynamicFetchers(event.Resource.GetName())
				s.notifyDiscoveryConfigChanged()
			default:
				s.Log.Warnf("Skipping unknown event type %s", event.Type)
			}
		case <-s.dynamicMatcherWatcher.Done():
			s.Log.Warnf("dynamic matcher watcher error: %v", s.dynamicMatcherWatcher.Error())
			return
		}
	}
}

// newDiscoveryConfigChangedSub creates a new subscription for DiscoveryConfig events.
// The consumer must have an active reader on the returned channel, and start a new Poll when it returns a value.
func (s *Server) newDiscoveryConfigChangedSub() (ch chan struct{}) {
	chSubscription := make(chan struct{}, 1)
	s.triggerFetchMu.Lock()
	s.TriggerFetchC = append(s.TriggerFetchC, chSubscription)
	s.triggerFetchMu.Unlock()
	return chSubscription
}

// triggerPoll sends a notification to all the registered watchers so that they start a new Poll.
func (s *Server) notifyDiscoveryConfigChanged() {
	s.triggerFetchMu.RLock()
	defer s.triggerFetchMu.RUnlock()
	for _, watcherTriggerC := range s.TriggerFetchC {
		select {
		case watcherTriggerC <- struct{}{}:
			// Successfully sent notification.
		default:
			// Channel already has valued queued.
		}
	}
}

func (s *Server) deleteDynamicFetchers(name string) {
	s.muDynamicDatabaseFetchers.Lock()
	delete(s.dynamicDatabaseFetchers, name)
	s.muDynamicDatabaseFetchers.Unlock()

	s.muDynamicServerAWSFetchers.Lock()
	delete(s.dynamicServerAWSFetchers, name)
	s.muDynamicServerAWSFetchers.Unlock()

	s.muDynamicServerAzureFetchers.Lock()
	delete(s.dynamicServerAzureFetchers, name)
	s.muDynamicServerAzureFetchers.Unlock()

	s.muDynamicServerGCPFetchers.Lock()
	delete(s.dynamicServerGCPFetchers, name)
	s.muDynamicServerGCPFetchers.Unlock()

	s.muDynamicTAGSyncFetchers.Lock()
	delete(s.dynamicTAGSyncFetchers, name)
	s.muDynamicTAGSyncFetchers.Unlock()

	s.muDynamicKubeIntegrationFetchers.Lock()
	delete(s.dynamicKubeIntegrationFetchers, name)
	s.muDynamicKubeIntegrationFetchers.Unlock()
}

// upsertDynamicMatchers upserts the internal set of dynamic matchers given a particular discovery config.
func (s *Server) upsertDynamicMatchers(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) error {
	matchers := Matchers{
		AWS:         dc.Spec.AWS,
		Azure:       dc.Spec.Azure,
		GCP:         dc.Spec.GCP,
		Kubernetes:  dc.Spec.Kube,
		AccessGraph: dc.Spec.AccessGraph,
	}
	s.discardUnsupportedMatchers(&matchers)

	awsServerFetchers, err := s.awsServerFetchersFromMatchers(s.ctx, matchers.AWS)
	if err != nil {
		return trace.Wrap(err)
	}
	s.muDynamicServerAWSFetchers.Lock()
	s.dynamicServerAWSFetchers[dc.GetName()] = awsServerFetchers
	s.muDynamicServerAWSFetchers.Unlock()

	azureServerFetchers := s.azureServerFetchersFromMatchers(matchers.Azure)
	s.muDynamicServerAzureFetchers.Lock()
	s.dynamicServerAzureFetchers[dc.GetName()] = azureServerFetchers
	s.muDynamicServerAzureFetchers.Unlock()

	gcpServerFetchers, err := s.gcpServerFetchersFromMatchers(s.ctx, matchers.GCP)
	if err != nil {
		return trace.Wrap(err)
	}
	s.muDynamicServerGCPFetchers.Lock()
	s.dynamicServerGCPFetchers[dc.GetName()] = gcpServerFetchers
	s.muDynamicServerGCPFetchers.Unlock()

	databaseFetchers, err := s.databaseFetchersFromMatchers(matchers)
	if err != nil {
		return trace.Wrap(err)
	}

	s.muDynamicDatabaseFetchers.Lock()
	s.dynamicDatabaseFetchers[dc.GetName()] = databaseFetchers
	s.muDynamicDatabaseFetchers.Unlock()

	awsSyncMatchers, err := s.accessGraphFetchersFromMatchers(
		ctx, matchers,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	s.muDynamicTAGSyncFetchers.Lock()
	s.dynamicTAGSyncFetchers[dc.GetName()] = awsSyncMatchers
	s.muDynamicTAGSyncFetchers.Unlock()

	kubeIntegrationFetchers, err := s.kubeIntegrationFetchersFromMatchers(matchers)
	if err != nil {
		return trace.Wrap(err)
	}

	s.muDynamicKubeIntegrationFetchers.Lock()
	s.dynamicKubeIntegrationFetchers[dc.GetName()] = kubeIntegrationFetchers
	s.muDynamicKubeIntegrationFetchers.Unlock()

	// TODO(marco): add other fetchers: Kube Clusters and Kube Resources (Apps)
	return nil
}

// discardUnsupportedMatchers drops any matcher that is not supported in the current DiscoveryService.
// Discarded Matchers:
// - when running in IntegrationOnlyCredentials mode, any Matcher that doesn't have an Integration is discarded.
func (s *Server) discardUnsupportedMatchers(m *Matchers) {
	if !s.IntegrationOnlyCredentials {
		return
	}

	// Discard all matchers that don't have an Integration
	validAWSMatchers := make([]types.AWSMatcher, 0, len(m.AWS))
	for i, m := range m.AWS {
		if m.Integration == "" {
			s.Log.Warnf("discarding AWS matcher [%d] - missing integration", i)
			continue
		}
		validAWSMatchers = append(validAWSMatchers, m)
	}
	m.AWS = validAWSMatchers

	if len(m.GCP) > 0 {
		s.Log.Warnf("discarding GCP matchers - missing integration")
		m.GCP = []types.GCPMatcher{}
	}

	if len(m.Azure) > 0 {
		s.Log.Warnf("discarding Azure matchers - missing integration")
		m.Azure = []types.AzureMatcher{}
	}

	if len(m.Kubernetes) > 0 {
		s.Log.Warnf("discarding Kubernetes matchers - missing integration")
		m.Kubernetes = []types.KubernetesMatcher{}
	}
}

// Stop stops the discovery service.
func (s *Server) Stop() {
	s.cancelfn()
	if s.ec2Watcher != nil {
		s.ec2Watcher.Stop()
	}
	if s.azureWatcher != nil {
		s.azureWatcher.Stop()
	}
	if s.gcpWatcher != nil {
		s.gcpWatcher.Stop()
	}
	if s.dynamicMatcherWatcher != nil {
		if err := s.dynamicMatcherWatcher.Close(); err != nil {
			s.Log.Warnf("dynamic matcher watcher closing error: ", trace.Wrap(err))
		}
	}
}

// Wait will block while the server is running.
func (s *Server) Wait() error {
	<-s.ctx.Done()
	if err := s.ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) getAzureSubscriptions(ctx context.Context, subs []string) ([]string, error) {
	subscriptionIds := subs
	if slices.Contains(subs, types.Wildcard) {
		subsClient, err := s.CloudClients.GetAzureSubscriptionClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		subscriptionIds, err = subsClient.ListSubscriptionIDs(ctx)
		return subscriptionIds, trace.Wrap(err)
	}

	return subscriptionIds, nil
}

func (s *Server) initTeleportNodeWatcher() (err error) {
	s.nodeWatcher, err = services.NewNodeWatcher(s.ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:    teleport.ComponentDiscovery,
			Log:          s.Log,
			Client:       s.AccessPoint,
			MaxStaleness: time.Minute,
		},
	})

	return trace.Wrap(err)
}

// splitSlice splits a slice into two, by putting all elements that satisfy the
// provided check function in the first slice, while putting all other elements
// in the second slice.
func splitSlice(ss []string, check func(string) bool) (split, other []string) {
	for _, e := range ss {
		if check(e) {
			split = append(split, e)
		} else {
			other = append(other, e)
		}
	}
	return
}

// splitMatchers splits a set of matchers by checking the matcher type.
func splitMatchers[T types.Matcher](matchers []T, matcherTypeCheck func(string) bool) (split, other []T) {
	for _, matcher := range matchers {
		splitTypes, otherTypes := splitSlice(matcher.GetTypes(), matcherTypeCheck)

		if len(splitTypes) > 0 {
			newMatcher := matcher.CopyWithTypes(splitTypes).(T)
			split = append(split, newMatcher)
		}
		if len(otherTypes) > 0 {
			newMatcher := matcher.CopyWithTypes(otherTypes).(T)
			other = append(other, newMatcher)
		}
	}
	return
}

func (s *Server) updatesEmptyDiscoveryGroup(getter func() (types.ResourceWithLabels, error)) bool {
	if s.DiscoveryGroup == "" {
		return false
	}
	old, err := getter()
	if err != nil {
		return false
	}
	oldDiscoveryGroup, _ := old.GetLabel(types.TeleportInternalDiscoveryGroupName)
	return oldDiscoveryGroup == ""
}
