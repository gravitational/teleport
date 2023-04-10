/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package okta

import (
	"context"
	"crypto"
	"encoding/base64"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
)

// ProxyGetter is an interface for retrieving proxy IDs.
type ProxyGetter interface {
	GetProxyIDs() []string
}

// Config is the configuration for the Okta service.
type Config struct {
	// Log is the logger for the Okta config.
	Log *logrus.Entry

	// Clock is the clock to use for this service.
	Clock clockwork.Clock

	// Hostname is the hostname.
	Hostname string

	// HostID is the ID of this host.
	HostID string

	// RotationGetter gets the rotation for this server.
	RotationGetter services.RotationGetter

	// ProxyGetter returns a list of proxies.
	ProxyGetter ProxyGetter

	// Emitter is events emitter, used to submit discrete events
	Emitter apievents.Emitter

	// AccessPoint is the caching access point for the Okta service.
	AccessPoint auth.OktaAccessPoint

	// OnHeartbeat is called after every heartbeat. Used to update process state.
	OnHeartbeat func(error)

	// OktaAPIEndpoint is the API endpoint to use for interacting with Okta.
	OktaAPIEndpoint string

	// OktaAPIToken is the API token.
	OktaAPIToken string

	// TimeBetweenSyncs is the amount of time between synchronization calls.
	TimeBetweenSyncs time.Duration

	// BackendTasksPerSecond is the number of backend modifying tasks that can be run per second.
	BackendTasksPerSecond int
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, teleport.ComponentOkta)
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Hostname == "" {
		return trace.BadParameter("hostname is missing")
	}
	if c.HostID == "" {
		return trace.BadParameter("host ID is missing")
	}
	if c.RotationGetter == nil {
		return trace.BadParameter("rotation getter is missing")
	}
	if c.ProxyGetter == nil {
		c.ProxyGetter = reversetunnel.NewConnectedProxyGetter()
	}
	if c.Emitter == nil {
		return trace.BadParameter("emitter is missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point is missing")
	}
	if c.OnHeartbeat == nil {
		return trace.BadParameter("OnHeartbeat is missing")
	}
	if c.OktaAPIEndpoint == "" {
		return trace.BadParameter("Okta API endpoint is missing")
	}
	if c.OktaAPIToken == "" {
		return trace.BadParameter("Okta API token is missing")
	}
	if c.TimeBetweenSyncs == 0 {
		// Default to running every 2 minutes.
		c.TimeBetweenSyncs = 120 * time.Second
	}
	if c.BackendTasksPerSecond == 0 {
		// Default to running 5 backend tasks per second.
		c.BackendTasksPerSecond = 5
	}
	return nil
}

// oktaClient is an Okta client interface that can be mocked for testing.
type oktaClient interface {
	// iterateGroups will iterate over the list of all Okta groups.
	iterateGroups(context.Context, func(*okta.Group) error) error

	// iterateApps will iterate over the list of all Okta applications.
	iterateApps(context.Context, func(okta.App) error) error

	// getOrgURL will return the org URL for the client.
	orgURL() string
}

// Service is the Okta service.
type Service struct {
	log   logrus.FieldLogger
	clock clockwork.Clock

	hostname       string
	hostID         string
	rotationGetter services.RotationGetter
	proxyGetter    ProxyGetter

	accessPoint  auth.OktaAccessPoint
	onHeartbeat  func(error)
	client       oktaClient
	emitter      apievents.Emitter
	orgURL       string
	orgURLBase64 string

	// rateLimiter will rate limit backend interactions.
	rateLimiter *rate.Limiter

	// hash function for getting unique names from app IDs/app link names.
	hash crypto.Hash

	// Heartbeat monitoring
	heartbeatsMu sync.Mutex
	heartbeats   map[string]*srv.Heartbeat

	// groupsReconciler will reconcile groups discovered in Okta.
	groupsReconciler *services.Reconciler

	// groups is the current mapping of groups.
	groupsMu sync.RWMutex
	groups   map[string]types.UserGroup

	// newGroups is the mapping of groups discovered by Okta, not yet synchronized
	// to the group reconciler.
	newGroupsMu sync.RWMutex
	newGroups   map[string]types.UserGroup

	// appsReconciler will reconcile applications discovered in Okta.
	appsReconciler *services.Reconciler

	// apps is the current mapping of apps.
	appsMu sync.RWMutex
	apps   map[string]*types.AppV3

	// newApps is the mapping of apps discovered by Okta, not yet synchronzied
	// to the apps reconciler.
	newAppsMu sync.RWMutex
	newApps   map[string]*types.AppV3

	// Import Rule mapping for the Okta objects.
	groupIRMappingMu sync.RWMutex
	groupIRMapping   map[string]prioritizedLabels

	applicationIRMappingMu sync.RWMutex
	applicationIRMapping   map[string]prioritizedLabels

	timeBetweenSyncs time.Duration

	syncStoppedChCloser sync.Once
	syncStoppedCh       chan struct{}

	stopChCloser sync.Once
	stopCh       chan struct{}
}

// New will create a new Okta service.
func New(ctx context.Context, config Config) (*Service, error) {
	return newWithClientCreator(ctx, config, func(context.Context, Config) (oktaClient, error) {
		_, client, err := okta.NewClient(ctx,
			okta.WithOrgUrl(config.OktaAPIEndpoint),
			okta.WithToken(config.OktaAPIToken),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &wrappedClient{
			client:     client,
			oktaOrgURL: config.OktaAPIEndpoint,
		}, nil
	})
}

// oktaClientFn is a function interface for creating Okta client.
type oktaClientFn func(context.Context, Config) (oktaClient, error)

// newWithClientCreator will create a new Okta service with the given oktaClient.
func newWithClientCreator(ctx context.Context, config Config, creator oktaClientFn) (*Service, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := creator(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	orgURL := strings.TrimSuffix(client.orgURL(), "/")
	orgURLBase64 := base64.RawURLEncoding.EncodeToString([]byte(orgURL))

	s := &Service{
		log:                  config.Log,
		clock:                config.Clock,
		hostname:             config.Hostname,
		hostID:               config.HostID,
		rotationGetter:       config.RotationGetter,
		proxyGetter:          config.ProxyGetter,
		accessPoint:          config.AccessPoint,
		onHeartbeat:          config.OnHeartbeat,
		client:               client,
		emitter:              config.Emitter,
		orgURL:               orgURL,
		orgURLBase64:         orgURLBase64,
		rateLimiter:          rate.NewLimiter(rate.Every(time.Second/time.Duration(config.BackendTasksPerSecond)), 1),
		hash:                 crypto.SHA256,
		heartbeats:           map[string]*srv.Heartbeat{},
		groups:               map[string]types.UserGroup{},
		newGroups:            map[string]types.UserGroup{},
		apps:                 map[string]*types.AppV3{},
		newApps:              map[string]*types.AppV3{},
		groupIRMapping:       map[string]prioritizedLabels{},
		applicationIRMapping: map[string]prioritizedLabels{},
		timeBetweenSyncs:     config.TimeBetweenSyncs,
		syncStoppedCh:        make(chan struct{}, 1),
		stopCh:               make(chan struct{}, 1),
	}

	return s, nil
}

// Start will start the Okta service.
func (s *Service) Start(ctx context.Context) error {
	if err := s.startReconcilers(ctx); err != nil {
		return trace.Wrap(err)
	}

	go s.synchronizeLoop(ctx)
	return nil
}

// Wait will wait for the Okta service to complete.
func (s *Service) Wait(ctx context.Context) {
	select {
	case <-s.syncStoppedCh:
	case <-ctx.Done():
	}
}

// Shutdown will stop any processes that are currently running.
func (s *Service) Shutdown() error {
	s.stopChCloser.Do(func() { close(s.stopCh) })
	s.syncStoppedChCloser.Do(func() { close(s.stopCh) })

	s.heartbeatsMu.Lock()
	defer s.heartbeatsMu.Unlock()

	var errs []error
	for _, heartbeat := range s.heartbeats {
		if err := heartbeat.Close(); err != nil {
			s.log.Errorf("Unable to close heartbeat: %v", err)
		}
	}

	return trace.NewAggregate(errs...)
}

// Close cleans up any lingering resources.
func (s *Service) Close(ctx context.Context) error {
	var errs []error
	if services.ShouldDeleteServerHeartbeatsOnShutdown(ctx) {
		for appName := range s.heartbeats {
			if err := s.accessPoint.DeleteApplicationServer(ctx, defaults.Namespace, s.hostID, appName); err != nil {
				if !trace.IsNotFound(err) {
					errs = append(errs, err)
				}
			}
		}
	}

	return trace.NewAggregate(errs...)
}

// HandleConnection handles connections for Okta applications.
func (s *Service) HandleConnection(conn net.Conn) {
	// TODO(mdwn): Handle connections for incoming application requests.
	s.log.Warnf("Okta handle connection is not yet implemented, closing the incoming connection.")
	if err := conn.Close(); err != nil {
		s.log.Errorf("Error closing connection: %v", err)
	}
}
