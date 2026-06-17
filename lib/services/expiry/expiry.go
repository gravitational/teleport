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

package expiry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	semaphoreNameAccessRequest = "auth.expiry"
	semaphoreNameAppSession    = "auth.expiry.app_session"
	semaphoreExpiration        = time.Minute * 5

	// scanInterval is the interval at which the expiry checker scans for resources.
	scanInterval = time.Minute * 5

	// pendingRequestGracePeriod is a grace period specifically for pending access requests.
	// This is allowed because a request's expiry may be extended on approval.
	pendingRequestGracePeriod = time.Second * 40

	// maxExpiresPerCycle is an arbitrary limit on the number of resources to expire per cycle
	// to prevent any one auth server holding the lease for more than a couple of minutes.
	maxExpiresPerCycle = 120

	// metricsSubsystem groups Prometheus metrics emitted by this service.
	metricsSubsystem = "expiry_service"

	// metricLabelResourceKind partitions the gauges by resource kind
	// (e.g. "access_request", "app_session").
	metricLabelResourceKind = "resource_kind"
)

// expiryMetrics holds the Prometheus metrics emitted by the expiry service.
// A new instance is constructed in New() per Service so tests are isolated.
type expiryMetrics struct {
	// expiredBeforeScan is the number of expired resources observed at the
	// start of the most recent scan, partitioned by resource kind.
	expiredBeforeScan *prometheus.GaugeVec

	// expiredAfterScan is the number of expired resources still awaiting
	// deletion at the end of the most recent scan.
	expiredAfterScan *prometheus.GaugeVec
}

// newExpiryMetrics constructs a fresh set of unregistered metrics.
func newExpiryMetrics() *expiryMetrics {
	return &expiryMetrics{
		expiredBeforeScan: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "expired_resources_before_scan",
			Help:      "Number of expired resources at the start of scan",
		}, []string{metricLabelResourceKind}),
		expiredAfterScan: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: metricsSubsystem,
			Name:      "expired_resources_after_scan",
			Help:      "Number of expired resources remaining at the end of scan",
		}, []string{metricLabelResourceKind}),
	}
}

// AccessPoint is the API used by the expiry service.
type AccessPoint interface {
	// Semaphores provides semaphore operations
	types.Semaphores

	// ListExpiredAccessRequests lists all access requests that are expired. This is used by
	// the expiry service. Access requests expiration handling is done outside the backend
	// because we need to emit audit events on the access requests expiry.
	ListExpiredAccessRequests(ctx context.Context, limit int, pageToken string) ([]*types.AccessRequestV3, string, error)

	// DeleteAccessRequest deletes an access request.
	DeleteAccessRequest(ctx context.Context, reqID string) error

	// ListExpiredAppSessions lists all application sessions that are expired.
	ListExpiredAppSessions(ctx context.Context, limit int, pageToken string) ([]types.WebSession, string, error)

	// DeleteAppSession removes an application web session.
	DeleteAppSession(ctx context.Context, req types.DeleteAppSessionRequest) error
}

// Config provides configuration for the expiry server.
type Config struct {
	// Log is the logger.
	Log *slog.Logger
	// Emitter is an events emitter, used to submit discrete events.
	Emitter apievents.Emitter
	// AccessPoint provides backend operations.
	AccessPoint AccessPoint
	// HostID is a unique ID of this host.
	HostID string
	// EnableAppSessionExpiryService enables the app session expiry task. Must match
	// the IdentityService option.
	EnableAppSessionExpiryService bool
}

// CheckAndSetDefaults checks required fields and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Emitter == nil {
		return trace.BadParameter("no Emitter configured for expiry")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("no AccessPoint configured for expiry")
	}
	return nil
}

// expiryTask defines a task for expiring resources.
type expiryTask struct {
	semaphoreName string
	resourceKind  string
	intervalCfg   interval.Config
	processFunc   func(context.Context)
}

// Service is an expiry service.
type Service struct {
	*Config
	expiryTasks []expiryTask
	metrics     *expiryMetrics
}

// New initializes an expiry service.
func New(cfg *Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	m := newExpiryMetrics()
	if err := metrics.RegisterPrometheusCollectors(
		m.expiredBeforeScan,
		m.expiredAfterScan,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		Config:  cfg,
		metrics: m,
	}

	intervalCfg := interval.Config{
		Duration:      scanInterval,
		FirstDuration: retryutils.FullJitter(scanInterval),
		Jitter:        retryutils.SeventhJitter,
	}

	s.expiryTasks = []expiryTask{
		{
			semaphoreName: semaphoreNameAccessRequest,
			resourceKind:  types.KindAccessRequest,
			intervalCfg:   intervalCfg,
			processFunc:   s.processRequests,
		},
	}

	if cfg.EnableAppSessionExpiryService {
		s.expiryTasks = append(s.expiryTasks, expiryTask{
			semaphoreName: semaphoreNameAppSession,
			resourceKind:  types.KindAppSession,
			intervalCfg:   intervalCfg,
			processFunc:   s.processAppSessions,
		})
	}

	return s, nil
}

// Run starts the expiry service.
func (s *Service) Run(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)

	for _, t := range s.expiryTasks {
		g.Go(func() error {
			return s.run(gCtx, t)
		})
	}

	return g.Wait()
}

// run drives a single expiry task.
func (s *Service) run(ctx context.Context, task expiryTask) error {
	for {
		if err := s.runWithLock(ctx, task); err != nil && !errors.Is(err, context.Canceled) {
			s.Log.ErrorContext(ctx, "Expiry service failed", "error", err)
		}

		select {
		case <-ctx.Done():
			// If context was canceled, we should stop.
			return nil
		case <-time.After(semaphoreExpiration):
			// Otherwise, semaphore is likely not available. Wait and retry.
		}
	}
}

// runWithLock acquires a semaphore lock for the task and runs the loop.
func (s *Service) runWithLock(ctx context.Context, task expiryTask) error {
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.AccessPoint,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: task.resourceKind,
					SemaphoreName: task.semaphoreName,
					MaxLeases:     1,
					Holder:        s.HostID,
				},
				Expiry: semaphoreExpiration,
			},
			Retry: retryutils.LinearConfig{
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
	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "Error cleaning up semaphore.", "error", err)
		}
	}()

	err = s.loop(lease, task)
	return trace.Wrap(err)
}

// loop processes the expired resources on the configured interval.
func (s *Service) loop(ctx context.Context, task expiryTask) error {
	interval := interval.New(task.intervalCfg)
	defer interval.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-interval.Next():
			task.processFunc(ctx)
		}
	}
}

func (s *Service) processRequests(ctx context.Context) {
	s.Log.DebugContext(ctx, "Cleaning up expired access requests.")

	// expiredBefore counts requests waiting to be expired
	// requestsExpired counts only successful expirations
	expiredBefore := 0
	requestsExpired := 0
	defer func() {
		s.metrics.expiredBeforeScan.WithLabelValues(types.KindAccessRequest).Set(float64(expiredBefore))
		s.metrics.expiredAfterScan.WithLabelValues(types.KindAccessRequest).Set(float64(expiredBefore - requestsExpired))
	}()

	readTime := time.Now()
	for expiredAccessRequest, err := range clientutils.Resources(ctx, s.AccessPoint.ListExpiredAccessRequests) {
		if err != nil {
			s.Log.ErrorContext(ctx, "Error listing expired access requests.", "error", err)
			return
		}

		// Add grace period for pending access requests as expiry time may be extended on approval.
		if expiredAccessRequest.GetState() == types.RequestState_PENDING {
			expiry := expiredAccessRequest.Expiry().Add(pendingRequestGracePeriod)
			if !readTime.After(expiry) {
				continue
			}
		}
		expiredBefore++

		// Keep iterating to count remaining requests but stop expiring them.
		if expiredBefore > maxExpiresPerCycle {
			continue
		}

		s.Log.DebugContext(ctx, "Expiring access request.", "request", expiredAccessRequest.GetName())
		if err := s.expireRequest(ctx, expiredAccessRequest); err != nil {
			s.Log.ErrorContext(ctx, "Error expiring access request.", "error", err)
			continue
		}
		requestsExpired++
	}

	if expiredBefore > maxExpiresPerCycle {
		s.Log.WarnContext(ctx,
			"Expired access request count exceeded per-scan cap. Will continue in the next run.",
			"expired", expiredBefore,
			"processed", requestsExpired,
			"max_per_cycle", maxExpiresPerCycle,
		)
	}

	s.Log.DebugContext(ctx, "Successfully cleaned up expired access requests.", "count", requestsExpired)
}

func (s *Service) processAppSessions(ctx context.Context) {
	s.Log.DebugContext(ctx, "Cleaning up expired application sessions.")

	// expiredBefore counts app sessions waiting to be expired
	// sessionsExpired counts only successful expirations
	expiredBefore := 0
	sessionsExpired := 0
	defer func() {
		s.metrics.expiredBeforeScan.WithLabelValues(types.KindAppSession).Set(float64(expiredBefore))
		s.metrics.expiredAfterScan.WithLabelValues(types.KindAppSession).Set(float64(expiredBefore - sessionsExpired))
	}()

	for expiredSession, err := range clientutils.Resources(ctx, s.AccessPoint.ListExpiredAppSessions) {
		if err != nil {
			s.Log.ErrorContext(ctx, "Error listing expired application sessions.", "error", err)
			return
		}
		expiredBefore++

		// Keep iterating to count remaining sessions but stop expiring them.
		if expiredBefore > maxExpiresPerCycle {
			continue
		}

		s.Log.DebugContext(ctx, "Expiring application session.",
			"user", expiredSession.GetUser(),
			"session_id", expiredSession.GetName())

		if err := s.expireAppSession(ctx, expiredSession); err != nil {
			s.Log.ErrorContext(ctx, "Error expiring application session.", "error", err)
			continue
		}
		sessionsExpired++
	}

	if expiredBefore > maxExpiresPerCycle {
		s.Log.WarnContext(ctx,
			"Expired application session count exceeded per-scan cap. Will continue in the next run.",
			"expired", expiredBefore,
			"processed", sessionsExpired,
			"max_per_cycle", maxExpiresPerCycle,
		)
	}

	s.Log.DebugContext(ctx, "Successfully cleaned up expired application sessions.", "count", sessionsExpired)
}

func (s *Service) expireRequest(ctx context.Context, req types.AccessRequest) error {
	expiry := req.Expiry()
	event := &apievents.AccessRequestExpire{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestExpireEvent,
			Code: events.AccessRequestExpireCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		RequestID:      req.GetName(),
		ResourceExpiry: &expiry,
	}
	// Emit expiry event before deletion as event we know event is expired here
	// but the deletion may fail.
	if err := s.Emitter.EmitAuditEvent(ctx, event); err != nil {
		return trace.Wrap(err)
	}

	if err := s.AccessPoint.DeleteAccessRequest(ctx, req.GetName()); err != nil {
		if trace.IsNotFound(err) {
			s.Log.InfoContext(ctx, "access request was already deleted", "request", req.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	return nil
}

func (s *Service) expireAppSession(ctx context.Context, sess types.WebSession) error {
	event := &apievents.AppSessionExpire{
		Metadata: apievents.Metadata{
			Type: events.AppSessionExpireEvent,
			Code: events.AppSessionExpireCode,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: sess.GetName(),
		},
		UserMetadata: apievents.UserMetadata{
			User: sess.GetUser(),
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Expires: sess.GetExpiryTime(),
		},
	}

	// Extract identity from certificate for audit event when possible.
	// It is unlikely that certificate parsing or identity extraction fails, so this serves as a sanity check.
	cert, err := tlsca.ParseCertificatePEM(sess.GetTLSCert())
	if err != nil {
		s.Log.WarnContext(ctx, "Failed to parse application session TLS certificate for expiry event.", "session_id", sess.GetName(), "error", err)
	} else {
		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		if err != nil {
			s.Log.WarnContext(ctx, "Failed to extract identity from application session TLS certificate for expiry event.", "session_id", sess.GetName(), "error", err)
		} else {
			userMetadata := identity.GetUserMetadata()
			userMetadata.User = sess.GetUser()
			event.UserMetadata = userMetadata
			event.AppMetadata = apievents.AppMetadata{
				AppPublicAddr: identity.RouteToApp.PublicAddr,
				AppName:       identity.RouteToApp.Name,
				AppTargetPort: uint32(identity.RouteToApp.TargetPort),
			}
		}
	}

	// Emit expiry event before deletion as we know the session is expired here
	// but the deletion may fail.
	if err := s.Emitter.EmitAuditEvent(ctx, event); err != nil {
		return trace.Wrap(err)
	}

	if err := s.AccessPoint.DeleteAppSession(ctx, types.DeleteAppSessionRequest{
		SessionID: sess.GetName(),
	}); err != nil {
		if trace.IsNotFound(err) {
			s.Log.InfoContext(ctx, "application session was already deleted", "session_id", sess.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	return nil
}
