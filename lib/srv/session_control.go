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

package srv

import (
	"context"
	"io"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
)

var userSessionLimitHitCount = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: teleport.MetricUserMaxConcurrentSessionsHit,
		Help: "Number of times a user exceeded their max concurrent ssh connections",
	},
)

func init() {
	_ = metrics.RegisterPrometheusCollectors(userSessionLimitHitCount)
}

// LockEnforcer determines whether a lock is being enforced on the provided targets
type LockEnforcer interface {
	CheckLockInForce(mode constants.LockingMode, targets ...types.LockTarget) error
}

// SessionControllerConfig contains dependencies needed to
// create a SessionController
type SessionControllerConfig struct {
	// Semaphores is used to obtain a semaphore lock when max sessions are defined
	Semaphores types.Semaphores
	// AccessPoint is the cache used to get cluster information
	AccessPoint AccessPoint
	// LockEnforcer is used to determine if locks should prevent a session
	LockEnforcer LockEnforcer
	// Emitter is used to emit session rejection events
	Emitter apievents.Emitter
	// Component is the component running the session controller. Nodes and Proxies
	// have different flows
	Component string
	// Logger is used to emit log entries
	Logger *logrus.Entry
	// TracerProvider creates a tracer so that spans may be emitted
	TracerProvider oteltrace.TracerProvider
	// ServerID is the UUID of the server
	ServerID string
	// Clock used in tests to change time
	Clock clockwork.Clock

	tracer oteltrace.Tracer
}

// CheckAndSetDefaults ensures all the required dependencies were
// provided and sets any optional values to their defaults
func (c *SessionControllerConfig) CheckAndSetDefaults() error {
	if c.Semaphores == nil {
		return trace.BadParameter("Semaphores must be provided")
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("AccessPoint must be provided")
	}

	if c.LockEnforcer == nil {
		return trace.BadParameter("LockWatcher must be provided")
	}

	if c.Emitter == nil {
		return trace.BadParameter("Emitter must be provided")
	}

	if c.Component == "" {
		return trace.BadParameter("Component must be provided")
	}

	if c.TracerProvider == nil {
		c.TracerProvider = tracing.DefaultProvider()
	}

	if c.Logger == nil {
		c.Logger = logrus.WithField(teleport.ComponentKey, "SessionCtrl")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	c.tracer = c.TracerProvider.Tracer("SessionController")

	return nil
}

// SessionController enforces session control restrictions required by
// locks, private key policy, and max connection limits
type SessionController struct {
	cfg SessionControllerConfig
}

// NewSessionController creates a SessionController from the provided config. If any
// of the required parameters in the SessionControllerConfig are not provided an
// error is returned.
func NewSessionController(cfg SessionControllerConfig) (*SessionController, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SessionController{cfg: cfg}, nil
}

// WebSessionContext contains information associated with a session
// established via the web ui.
type WebSessionContext interface {
	GetUserAccessChecker() (services.AccessChecker, error)
	GetSSHCertificate() (*ssh.Certificate, error)
	GetUser() string
}

// WebSessionController is a wrapper around [SessionController] which can be
// used to create an [IdentityContext] and apply session controls for a web session.
// This allows `lib/web` to not depend on `lib/srv`.
func WebSessionController(controller *SessionController) func(ctx context.Context, sctx WebSessionContext, login, localAddr, remoteAddr string) (context.Context, error) {
	return func(ctx context.Context, sctx WebSessionContext, login, localAddr, remoteAddr string) (context.Context, error) {
		accessChecker, err := sctx.GetUserAccessChecker()
		if err != nil {
			return ctx, trace.Wrap(err)
		}

		sshCert, err := sctx.GetSSHCertificate()
		if err != nil {
			return ctx, trace.Wrap(err)
		}

		unmappedRoles, err := services.ExtractRolesFromCert(sshCert)
		if err != nil {
			return ctx, trace.Wrap(err)
		}

		accessRequestIDs, err := ParseAccessRequestIDs(sshCert.Extensions[teleport.CertExtensionTeleportActiveRequests])
		if err != nil {
			return ctx, trace.Wrap(err)
		}

		identity := IdentityContext{
			AccessChecker:  accessChecker,
			TeleportUser:   sctx.GetUser(),
			Login:          login,
			Certificate:    sshCert,
			UnmappedRoles:  unmappedRoles,
			ActiveRequests: accessRequestIDs,
			Impersonator:   sshCert.Extensions[teleport.CertExtensionImpersonator],
		}
		ctx, err = controller.AcquireSessionContext(ctx, identity, localAddr, remoteAddr)
		return ctx, trace.Wrap(err)
	}
}

// AcquireSessionContext attempts to create a context for the session. If the session is
// not allowed due to session control an error is returned. The returned
// context is scoped to the session and will be canceled in the event the semaphore lock
// is no longer held. The closers provided are immediately closed when the semaphore lock
// is released as well.
func (s *SessionController) AcquireSessionContext(ctx context.Context, identity IdentityContext, localAddr, remoteAddr string, closers ...io.Closer) (context.Context, error) {
	// create a separate context for tracing the operations
	// within that doesn't leak into the returned context
	spanCtx, span := s.cfg.tracer.Start(ctx, "SessionController/AcquireSessionContext")
	defer span.End()

	authPref, err := s.cfg.AccessPoint.GetAuthPreference(spanCtx)
	if err != nil {
		return ctx, trace.Wrap(err)
	}

	clusterName, err := s.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return ctx, trace.Wrap(err)
	}

	lockingMode := identity.AccessChecker.LockingMode(authPref.GetLockingMode())
	lockTargets := ComputeLockTargets(clusterName.GetClusterName(), s.cfg.ServerID, identity)

	if lockErr := s.cfg.LockEnforcer.CheckLockInForce(lockingMode, lockTargets...); lockErr != nil {
		s.emitRejection(spanCtx, identity.GetUserMetadata(), localAddr, remoteAddr, lockErr.Error(), 0)
		return ctx, trace.Wrap(lockErr)
	}

	// Check that the required private key policy, defined by roles and auth pref,
	// is met by this Identity's ssh certificate.
	identityPolicy := keys.PrivateKeyPolicy(identity.Certificate.Extensions[teleport.CertExtensionPrivateKeyPolicy])
	requiredPolicy, err := identity.AccessChecker.PrivateKeyPolicy(authPref.GetPrivateKeyPolicy())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !requiredPolicy.IsSatisfiedBy(identityPolicy) {
		return ctx, keys.NewPrivateKeyPolicyError(requiredPolicy)
	}

	// Don't apply the following checks in non-node contexts.
	if s.cfg.Component != teleport.ComponentNode {
		return ctx, nil
	}

	// Device Trust: authorize device extensions.
	if err := dtauthz.VerifySSHUser(authPref.GetDeviceTrust(), identity.Certificate); err != nil {
		return ctx, trace.Wrap(err)
	}

	ctx, err = s.EnforceConnectionLimits(
		ctx,
		auth.ConnectionIdentity{
			Username:       identity.TeleportUser,
			MaxConnections: identity.AccessChecker.MaxConnections(),
			LocalAddr:      localAddr,
			RemoteAddr:     remoteAddr,
			UserMetadata:   identity.GetUserMetadata(),
		},
		closers...,
	)
	return ctx, trace.Wrap(err)
}

// EnforceConnectionLimits retrieves a semaphore lock to ensure that connection limits
// for the identity are enforced. If the lock is closed for any reason prior to the connection
// being terminated any of the provided closers will be closed.
func (s *SessionController) EnforceConnectionLimits(ctx context.Context, identity auth.ConnectionIdentity, closers ...io.Closer) (context.Context, error) {
	maxConnections := identity.MaxConnections
	if maxConnections == 0 {
		// concurrent session control is not active, nothing
		// else needs to be done here.
		return ctx, nil
	}

	netConfig, err := s.cfg.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return ctx, trace.Wrap(err)
	}

	semLock, err := services.AcquireSemaphoreLock(ctx, services.SemaphoreLockConfig{
		Service: s.cfg.Semaphores,
		Clock:   s.cfg.Clock,
		Expiry:  netConfig.GetSessionControlTimeout(),
		Params: types.AcquireSemaphoreRequest{
			SemaphoreKind: types.SemaphoreKindConnection,
			SemaphoreName: identity.Username,
			MaxLeases:     maxConnections,
			Holder:        s.cfg.ServerID,
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), teleport.MaxLeases) {
			// user has exceeded their max concurrent ssh connections.
			userSessionLimitHitCount.Inc()
			s.emitRejection(ctx, identity.UserMetadata, identity.LocalAddr, identity.RemoteAddr, events.SessionRejectedEvent, maxConnections)

			return ctx, trace.AccessDenied("too many concurrent ssh connections for user %q (max=%d)", identity.Username, maxConnections)
		}

		return ctx, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	// ensure that losing the lock closes the connection context.  Under normal
	// conditions, cancellation propagates from the connection context to the
	// lock, but if we lose the lock due to some error (e.g. poor connectivity
	// to auth server) then cancellation propagates in the other direction.
	go func() {
		// TODO(fspmarshall): If lock was lost due to error, find a way to propagate
		// an error message to user.
		<-semLock.Done()
		cancel()

		// close any provided closers
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()

	return ctx, nil
}

// emitRejection emits a SessionRejectedEvent with the provided information
func (s *SessionController) emitRejection(ctx context.Context, userMetadata apievents.UserMetadata, localAddr, remoteAddr string, reason string, max int64) {
	// link a background context to the current span so things
	// are related but while still allowing the audit event to
	// not be tied to the request scoped context
	emitCtx := oteltrace.ContextWithSpanContext(context.Background(), oteltrace.SpanContextFromContext(ctx))

	ctx, span := s.cfg.tracer.Start(emitCtx, "SessionController/emitRejection")
	defer span.End()

	if err := s.cfg.Emitter.EmitAuditEvent(ctx, &apievents.SessionReject{
		Metadata: apievents.Metadata{
			Type: events.SessionRejectedEvent,
			Code: events.SessionRejectedCode,
		},
		UserMetadata: userMetadata,
		ConnectionMetadata: apievents.ConnectionMetadata{
			Protocol:   events.EventProtocolSSH,
			LocalAddr:  localAddr,
			RemoteAddr: remoteAddr,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        s.cfg.ServerID,
			ServerNamespace: apidefaults.Namespace,
		},
		Reason:  reason,
		Maximum: max,
	}); err != nil {
		s.cfg.Logger.WithError(err).Warn("Failed to emit session reject event.")
	}
}
