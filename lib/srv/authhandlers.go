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
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auditd"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/connectmycomputer"
	"github.com/gravitational/teleport/lib/decision"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var (
	failedLoginCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricFailedLoginAttempts,
			Help: "Number of times there was a failed login",
		},
	)

	certificateMismatchCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricCertificateMismatch,
			Help: "Number of times there was a certificate mismatch",
		},
	)

	prometheusCollectors = []prometheus.Collector{failedLoginCount, certificateMismatchCount}
)

var errRoleFileCopyingNotPermitted = trace.AccessDenied("file copying via SCP or SFTP is not permitted")

// AuthHandlerConfig is the configuration for an application handler.
type AuthHandlerConfig struct {
	// Server is the services.Server in the backend.
	Server Server

	// Component is the type of SSH server (node, proxy, or recording proxy).
	Component string

	// Emitter is event emitter
	Emitter apievents.Emitter

	// AccessPoint is used to access the Auth Server.
	AccessPoint AccessPoint

	// TargetServer is the host that the connection is being established for.
	// It **MUST** only be populated when the target is a teleport ssh server
	// or an agentless server.
	TargetServer types.Server

	// FIPS mode means Teleport started in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification.
	// Defaults to real clock if unspecified
	Clock clockwork.Clock

	// OnRBACFailure is an opitonal callback used to hook in metrics/logs related to
	// RBAC failures.
	OnRBACFailure func(conn ssh.ConnMetadata, ident *sshca.Identity, err error)
}

func (c *AuthHandlerConfig) CheckAndSetDefaults() error {
	if c.Server == nil {
		return trace.BadParameter("Server required")
	}

	if c.Emitter == nil {
		return trace.BadParameter("Emitter required")
	}

	if c.AccessPoint == nil {
		return trace.BadParameter("AccessPoint required")
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

// AuthHandlers are common authorization and authentication related handlers
// used by the regular and forwarding server.
type AuthHandlers struct {
	loginChecker
	proxyingChecker
	gitForwardingChecker

	log *slog.Logger

	c *AuthHandlerConfig
}

// NewAuthHandlers initializes authorization and authentication handlers
func NewAuthHandlers(config *AuthHandlerConfig) (*AuthHandlers, error) {
	if err := metrics.RegisterPrometheusCollectors(prometheusCollectors...); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ah := &AuthHandlers{
		c:   config,
		log: slog.With(teleport.ComponentKey, config.Component),
	}
	lc := &ahLoginChecker{
		log: ah.log,
		c:   ah.c,
	}

	ah.loginChecker = lc
	ah.proxyingChecker = lc
	ah.gitForwardingChecker = lc

	return ah, nil
}

// CreateIdentityContext returns an IdentityContext populated with information
// about the logged in user on the connection.
func (h *AuthHandlers) CreateIdentityContext(sconn *ssh.ServerConn) (IdentityContext, error) {
	certRaw := []byte(sconn.Permissions.Extensions[utils.CertTeleportUserCertificate])
	certificate, err := apisshutils.ParseCertificate(certRaw)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	var permitCount int
	var accessPermit *decisionpb.SSHAccessPermit
	if permitRaw, ok := sconn.Permissions.Extensions[utils.ExtIntSSHAccessPermit]; ok {
		accessPermit = &decisionpb.SSHAccessPermit{}
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(permitRaw), accessPermit); err != nil {
			return IdentityContext{}, trace.Wrap(err)
		}
		permitCount++
	}

	var proxyPermit *proxyingPermit
	if permitRaw, ok := sconn.Permissions.Extensions[utils.ExtIntProxyingPermit]; ok {
		proxyPermit = &proxyingPermit{}
		if err := utils.FastUnmarshal([]byte(permitRaw), proxyPermit); err != nil {
			return IdentityContext{}, trace.Wrap(err)
		}
		permitCount++
	}

	var gitForwardingPermit *GitForwardingPermit
	if permitRaw, ok := sconn.Permissions.Extensions[utils.ExtIntGitForwardingPermit]; ok {
		gitForwardingPermit = &GitForwardingPermit{}
		if err := utils.FastUnmarshal([]byte(permitRaw), gitForwardingPermit); err != nil {
			return IdentityContext{}, trace.Wrap(err)
		}
		permitCount++
	}

	// verify that exactly one permit was defined
	if permitCount != 1 {
		return IdentityContext{}, trace.BadParameter("identity context expected exactly one permit, got %d (this is a bug)", permitCount)
	}

	unmappedIdentity, err := sshca.DecodeIdentity(certificate)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	var certValidBefore time.Time
	if unmappedIdentity.ValidBefore != 0 {
		certValidBefore = time.Unix(int64(unmappedIdentity.ValidBefore), 0)
	}

	certAuthority, err := h.authorityForCert(types.UserCA, certificate.SignatureKey)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	clusterName, err := h.c.AccessPoint.GetClusterName(context.TODO())
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	accessInfo, err := fetchAccessInfo(unmappedIdentity, certAuthority, clusterName.GetClusterName())
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}
	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), h.c.AccessPoint)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	return IdentityContext{
		UnmappedIdentity:                    unmappedIdentity,
		AccessPermit:                        accessPermit,
		ProxyingPermit:                      proxyPermit,
		GitForwardingPermit:                 gitForwardingPermit,
		Login:                               sconn.User(),
		CertAuthority:                       certAuthority,
		UnstableSessionJoiningAccessChecker: accessChecker,
		UnstableClusterAccessChecker:        accessChecker.CheckAccessToRemoteCluster,
		TeleportUser:                        unmappedIdentity.Username,
		RouteToCluster:                      unmappedIdentity.RouteToCluster,
		UnmappedRoles:                       unmappedIdentity.Roles,
		CertValidBefore:                     certValidBefore,
		Impersonator:                        unmappedIdentity.Impersonator,
		ActiveRequests:                      unmappedIdentity.ActiveRequests,
		DisallowReissue:                     unmappedIdentity.DisallowReissue,
		Renewable:                           unmappedIdentity.Renewable,
		BotName:                             unmappedIdentity.BotName,
		BotInstanceID:                       unmappedIdentity.BotInstanceID,
		PreviousIdentityExpires:             unmappedIdentity.PreviousIdentityExpires,
	}, nil
}

// CheckAgentForward checks if agent forwarding is allowed for the users RoleSet.
func (h *AuthHandlers) CheckAgentForward(ctx *ServerContext) error {
	if ctx.Identity.AccessPermit != nil && ctx.Identity.AccessPermit.ForwardAgent {
		return nil
	}

	if ctx.Identity.ProxyingPermit != nil && h.c.Component == teleport.ComponentProxy {
		// we are a proxy and not the access-controlling boundary. allow agent forwarding
		// in order to ensure that session recording functions correctly. Note that it is
		// the ForwardingNode component that actually does session recording, but the
		// proxy component is the one that wants agent forwarding enabled in order to set up the
		// prerequisite conditions for recording.
		return nil
	}

	return trace.AccessDenied("agent forwarding not permitted")
}

// CheckX11Forward checks if X11 forwarding is permitted for the user's RoleSet.
func (h *AuthHandlers) CheckX11Forward(ctx *ServerContext) error {
	if ctx.Identity.AccessPermit != nil && ctx.Identity.AccessPermit.X11Forwarding {
		return nil
	}

	if ctx.Identity.ProxyingPermit != nil && h.c.Component == teleport.ComponentForwardingNode {
		// we are a proxy and not the access-controlling boundary. Allow X11 forwarding requests to pass through
		// the recording layer and down to the enforcing node.
		return nil
	}

	return trace.AccessDenied("x11 forwarding not permitted")
}

func (h *AuthHandlers) CheckFileCopying(ctx *ServerContext) error {
	if ctx.Identity.AccessPermit != nil && ctx.Identity.AccessPermit.SshFileCopy {
		return nil
	}

	return trace.Wrap(errRoleFileCopyingNotPermitted)
}

// CheckPortForward checks if port forwarding is allowed for the users RoleSet.
func (h *AuthHandlers) CheckPortForward(addr string, ctx *ServerContext, requestedMode decisionpb.SSHPortForwardMode) error {
	if ctx.Identity.AccessPermit == nil {
		return trace.AccessDenied("port forwarding not permitted")
	}

	allowedMode := ctx.Identity.AccessPermit.PortForwardMode
	if allowedMode == decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_ON {
		return nil
	}

	if allowedMode == decisionpb.SSHPortForwardMode_SSH_PORT_FORWARD_MODE_OFF || allowedMode != requestedMode {
		systemErrorMessage := fmt.Sprintf("port forwarding not allowed for user: %v", ctx.Identity.TeleportUser)
		userErrorMessage := "port forwarding not allowed"

		// Emit port forward failure event
		if err := h.c.Emitter.EmitAuditEvent(h.c.Server.Context(), &apievents.PortForward{
			Metadata: apievents.Metadata{
				Type: events.PortForwardEvent,
				Code: events.PortForwardFailureCode,
			},
			UserMetadata: ctx.Identity.GetUserMetadata(),
			ConnectionMetadata: apievents.ConnectionMetadata{
				LocalAddr:  ctx.ServerConn.LocalAddr().String(),
				RemoteAddr: ctx.ServerConn.RemoteAddr().String(),
			},
			Addr: addr,
			Status: apievents.Status{
				Success: false,
				Error:   systemErrorMessage,
			},
		}); err != nil {
			h.log.WarnContext(h.c.Server.Context(), "Failed to emit port forward deny audit event", "error", err)
		}

		h.log.WarnContext(h.c.Server.Context(), "Port forwarding request denied", "error", systemErrorMessage)

		return trace.AccessDenied("%s", userErrorMessage)
	}

	return nil
}

// UserKeyAuth implements SSH client authentication using public keys and is
// called by the server every time the client connects.
func (h *AuthHandlers) UserKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (ppms *ssh.Permissions, rerr error) {
	ctx := context.Background()

	fingerprint := fmt.Sprintf("%v %v", key.Type(), sshutils.Fingerprint(key))

	// create a new logging entry with info specific to this login attempt
	log := h.log.With(
		"local_addr", conn.LocalAddr(),
		"remote_addr", conn.RemoteAddr(),
		"user", conn.User(),
		"fingerprint", fingerprint,
	)

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		log.DebugContext(ctx, "rejecting auth attempt, unsupported key type")
		return nil, trace.BadParameter("unsupported key type: %v", fingerprint)
	}

	log.DebugContext(ctx, "processing auth attempt with key",
		slog.Group("cert",
			"serial", cert.Serial,
			"type", cert.CertType,
			"key_id", cert.KeyId,
			"valid_principals", cert.ValidPrincipals,
			"valid_after", cert.ValidAfter,
			"valid_before", cert.ValidBefore,
			"permissions", cert.Permissions,
			"reserved", cert.Reserved,
		),
	)

	if len(cert.ValidPrincipals) == 0 {
		log.DebugContext(ctx, "rejecting auth attempt without valid principals")
		return nil, trace.BadParameter("need a valid principal for key %v", fingerprint)
	}
	if len(cert.KeyId) == 0 {
		log.DebugContext(ctx, "rejecting auth attempt without valid key ID")
		return nil, trace.BadParameter("need a valid key for key %v", fingerprint)
	}

	ident, err := sshca.DecodeIdentity(cert)
	if err != nil {
		log.WarnContext(ctx, "failed to decode ssh identity from cert", "error", err)
		return nil, trace.BadParameter("failed to decode ssh identity from cert: %v", fingerprint)
	}

	// only failed attempts are logged right now
	recordFailedLogin := func(err error) {
		failedLoginCount.Inc()
		_, isConnectMyComputerNode := h.c.Server.GetInfo().GetLabel(types.ConnectMyComputerNodeOwnerLabel)
		principal := conn.User()

		message := fmt.Sprintf("Principal %q is not allowed by this certificate. Ensure your roles grants access by adding it to the 'login' property.", principal)
		if isConnectMyComputerNode {
			// This message ends up being used only when the cert does not include the principal in the
			// role, not when the principal is denied by a role.
			//
			// It's unlikely we'll ever run into this scenario as the connection test UI for Connect My
			// Computer lets the user select only among the logins defined within the Connect My Computer
			// role. It fails early if the list of logins is empty or if the user does not hold the
			// Connect My Computer role.
			//
			// The only way this could happen is if the backend state got updated between fetching the
			// logins from the role and actually performing the test.
			connectMyComputerRoleName := connectmycomputer.GetRoleNameForUser(ident.Username)

			message = fmt.Sprintf("Principal %q is not allowed by this certificate. Ensure that the role %q includes %q in the 'login' property. ",
				principal, connectMyComputerRoleName, principal) +
				"Removing the agent in Teleport Connect and starting the Connect My Computer setup again should fix this problem."
		}
		traceType := types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL

		if trace.IsAccessDenied(err) {
			message = "You are not authorized to access this node. Ensure your role grants access by adding it to the 'node_labels' property."
			if isConnectMyComputerNode {
				// It's more likely that a role denies the login rather than node_labels matching
				// types.ConnectMyComputerNodeOwnerLabel. If a role denies access to the Connect My Computer
				// node through node_labels, the user would never be able to see that the node has joined
				// the cluster and would not be able to get to the connection test step.
				connectMyComputerRoleName := connectmycomputer.GetRoleNameForUser(ident.Username)
				nodeLabel := fmt.Sprintf("%s: %s", types.ConnectMyComputerNodeOwnerLabel, ident.Username)
				message = fmt.Sprintf(
					"You are not authorized to access this node. Ensure that you hold the role %q and that "+
						"no role denies you access to the login %q and to nodes labeled with %q.",
					connectMyComputerRoleName, principal, nodeLabel)
			}

			traceType = types.ConnectionDiagnosticTrace_RBAC_NODE
		}

		if err := h.maybeAppendDiagnosticTrace(ctx, ident.ConnectionDiagnosticID,
			traceType,
			message,
			err,
		); err != nil {
			h.log.WarnContext(ctx, "Failed to append Trace to ConnectionDiagnostic", "error", err)
		}

		if err := h.c.Emitter.EmitAuditEvent(h.c.Server.Context(), &apievents.AuthAttempt{
			Metadata: apievents.Metadata{
				Type: events.AuthAttemptEvent,
				Code: events.AuthAttemptFailureCode,
			},
			UserMetadata: apievents.UserMetadata{
				Login:         principal,
				User:          ident.Username,
				TrustedDevice: ident.GetDeviceMetadata(),
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				LocalAddr:  conn.LocalAddr().String(),
				RemoteAddr: conn.RemoteAddr().String(),
			},
			Status: apievents.Status{
				Success: false,
				Error:   err.Error(),
			},
		}); err != nil {
			h.log.WarnContext(ctx, "Failed to emit failed login audit event", "error", err)
		}

		auditdMsg := auditd.Message{
			SystemUser:   principal,
			TeleportUser: ident.Username,
			ConnAddress:  conn.RemoteAddr().String(),
		}

		if err := auditd.SendEvent(auditd.AuditUserErr, auditd.Failed, auditdMsg); err != nil {
			log.WarnContext(ctx, "Failed to send an event to auditd", "error", err)
		}
	}

	// Check that the user certificate uses supported public key algorithms, was
	// issued by Teleport, and check the certificate metadata (principals,
	// timestamp, etc). Fallback to keys is not supported.
	certChecker := apisshutils.CertChecker{
		CertChecker: ssh.CertChecker{
			IsUserAuthority: h.IsUserAuthority,
			Clock:           h.c.Clock.Now,
		},
		FIPS: h.c.FIPS,
	}

	originalPermissions, err := certChecker.Authenticate(conn, key)
	if err != nil {
		certificateMismatchCount.Inc()
		recordFailedLogin(err)
		return nil, trace.Wrap(err)
	}
	log.DebugContext(ctx, "Successfully authenticated")

	for ext := range originalPermissions.Extensions {
		if utils.IsInternalSSHExtension(ext) {
			return nil, trace.BadParameter("internal extension %q is not permitted in cert permissions", ext)
		}
	}

	for ext := range originalPermissions.CriticalOptions {
		if utils.IsInternalSSHExtension(ext) {
			return nil, trace.BadParameter("internal extension %q is not permitted in cert critical options", ext)
		}
	}

	clusterName, err := h.c.AccessPoint.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var certType string
	switch ident.CertType {
	case ssh.UserCert:
		certType = utils.ExtIntCertTypeUser
	case ssh.HostCert:
		certType = utils.ExtIntCertTypeHost
	default:
		log.WarnContext(ctx, "Received unexpected cert type", "cert_type", cert.CertType)
		return nil, trace.BadParameter("unsupported cert type %v", ident.CertType)
	}

	// this is the only way we know of to pass valid additional data about the
	// connection to the handlers.
	outputPermissions := &ssh.Permissions{
		Extensions: map[string]string{
			utils.CertTeleportUser:            ident.Username,
			utils.CertTeleportClusterName:     clusterName.GetClusterName(),
			utils.CertTeleportUserCertificate: string(ssh.MarshalAuthorizedKey(cert)),
			utils.ExtIntCertType:              certType,
		},
	}

	// even if the returned CA isn't used when a RBAC check isn't
	// preformed, we still need to verify that User CA signed the
	// client's certificate
	ca, err := h.authorityForCert(types.UserCA, cert.SignatureKey)
	if err != nil {
		log.ErrorContext(ctx, "Permission denied", "error", err)
		recordFailedLogin(err)
		return nil, trace.Wrap(err)
	}

	// the git forwarding component currently only supports an authorization model that makes sense
	// for local identities. reject all non-local identities explicitly.
	if h.c.Component == teleport.ComponentForwardingGit && clusterName.GetClusterName() != ca.GetClusterName() {
		log.ErrorContext(ctx, "cross-cluster git forwarding is not supported", "local_cluster", clusterName.GetClusterName(), "remote_cluster", ca.GetClusterName())
		err = trace.AccessDenied("cross-cluster git forwarding is not supported")
		recordFailedLogin(err)
		return nil, err
	}

	var accessPermit *decisionpb.SSHAccessPermit
	var gitForwardingPermit *GitForwardingPermit
	var proxyPermit *proxyingPermit
	var diagnosticTracing bool

	switch h.c.Component {
	case teleport.ComponentForwardingGit:
		gitForwardingPermit, err = h.evaluateGitForwarding(ident, ca, clusterName.GetClusterName(), h.c.TargetServer)
	case teleport.ComponentProxy:
		proxyPermit, err = h.evaluateProxying(ident, ca, clusterName.GetClusterName())
	case teleport.ComponentForwardingNode:
		diagnosticTracing = true
		if h.c.TargetServer != nil && h.c.TargetServer.IsOpenSSHNode() {
			accessPermit, err = h.evaluateSSHAccess(ident, ca, clusterName.GetClusterName(), h.c.TargetServer, conn.User())
		} else {
			proxyPermit, err = h.evaluateProxying(ident, ca, clusterName.GetClusterName())
		}
	case teleport.ComponentNode:
		diagnosticTracing = true
		accessPermit, err = h.evaluateSSHAccess(ident, ca, clusterName.GetClusterName(), h.c.Server.GetInfo(), conn.User())
	default:
		return nil, trace.BadParameter("cannot determine appropriate authorization checks for unknown component %q (this is a bug)", h.c.Component)
	}

	if err != nil {
		log.ErrorContext(ctx, "permission denied",
			"error", err,
			"local_addr", logutils.StringerAttr(conn.LocalAddr()),
			"remote_addr", logutils.StringerAttr(conn.RemoteAddr()),
			"key", key.Type(),
			"fingerprint", sshutils.Fingerprint(key),
			"user", cert.KeyId,
		)

		recordFailedLogin(err)

		if h.c.OnRBACFailure != nil {
			h.c.OnRBACFailure(conn, ident, err)
		}

		return nil, trace.Wrap(err)
	}

	if accessPermit != nil {
		encodedPermit, err := protojson.Marshal(accessPermit)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		outputPermissions.Extensions[utils.ExtIntSSHAccessPermit] = string(encodedPermit)
	}

	if proxyPermit != nil {
		encodedPermit, err := utils.FastMarshal(proxyPermit)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		outputPermissions.Extensions[utils.ExtIntProxyingPermit] = string(encodedPermit)
	}

	if gitForwardingPermit != nil {
		encodedPermit, err := utils.FastMarshal(gitForwardingPermit)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		outputPermissions.Extensions[utils.ExtIntGitForwardingPermit] = string(encodedPermit)
	}

	if diagnosticTracing {
		if err := h.maybeAppendDiagnosticTrace(ctx, ident.ConnectionDiagnosticID,
			types.ConnectionDiagnosticTrace_RBAC_NODE,
			"You have access to the Node.",
			nil,
		); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := h.maybeAppendDiagnosticTrace(ctx, ident.ConnectionDiagnosticID,
			types.ConnectionDiagnosticTrace_CONNECTIVITY,
			"Node is alive and reachable.",
			nil,
		); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := h.maybeAppendDiagnosticTrace(ctx, ident.ConnectionDiagnosticID,
			types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
			"The requested principal is allowed.",
			nil,
		); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return outputPermissions, nil
}

func (h *AuthHandlers) maybeAppendDiagnosticTrace(ctx context.Context, connectionDiagnosticID string, traceType types.ConnectionDiagnosticTrace_TraceType, message string, traceError error) error {
	if connectionDiagnosticID == "" {
		return nil
	}

	connectionTrace := types.NewTraceDiagnosticConnection(traceType, message, traceError)

	_, err := h.c.AccessPoint.AppendDiagnosticTrace(ctx, connectionDiagnosticID, connectionTrace)
	return trace.Wrap(err)
}

// HostKeyAuth implements host key verification and is called by the client
// to validate the certificate presented by the target server. If the target
// server presents a SSH certificate, we validate that it was Teleport that
// generated the certificate. If the target server presents a public key, if
// we are strictly checking keys, we reject the target server. If we are not
// we take whatever.
func (h *AuthHandlers) HostKeyAuth(addr string, remote net.Addr, key ssh.PublicKey) error {
	// Check if the given host key was signed by a Teleport certificate
	// authority (CA) or fallback to host key checking if it's allowed.
	certChecker := apisshutils.CertChecker{
		CertChecker: ssh.CertChecker{
			IsHostAuthority: h.IsHostAuthority,
			HostKeyFallback: h.hostKeyCallback,
			Clock:           h.c.Clock.Now,
		},
		FIPS: h.c.FIPS,
	}
	err := certChecker.CheckHostKey(addr, remote, key)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// hostKeyCallback allows connections to hosts that present keys only if
// strict host key checking is disabled.
func (h *AuthHandlers) hostKeyCallback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	// Use the server's shutdown context.
	ctx := h.c.Server.Context()

	// For SubKindOpenSSHEICENode we use SSH Keys (EC2 does not support Certificates in ec2.SendSSHPublicKey).
	if h.c.Server.TargetMetadata().ServerSubKind == types.SubKindOpenSSHEICENode {
		return nil
	}

	// If strict host key checking is enabled, reject host key fallback.
	recConfig, err := h.c.AccessPoint.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if recConfig.GetProxyChecksHostKeys() {
		return trace.AccessDenied("remote host presented a public key, expected a host certificate")
	}

	// If strict host key checking is not enabled, log that Teleport trusted an
	// insecure key, but allow the request to go through.
	h.log.WarnContext(ctx, "Insecure configuration! Strict host key checking disabled, allowing login without checking host key", "key_type", key.Type())
	return nil
}

// IsUserAuthority is called during checking the client key, to see if the
// key used to sign the certificate was a Teleport CA.
func (h *AuthHandlers) IsUserAuthority(cert ssh.PublicKey) bool {
	if _, err := h.authorityForCert(types.UserCA, cert); err != nil {
		return false
	}

	return true
}

// IsHostAuthority is called when checking the host certificate a server
// presents. It make sure that the key used to sign the host certificate was a
// Teleport CA.
func (h *AuthHandlers) IsHostAuthority(cert ssh.PublicKey, address string) bool {
	if _, err := h.authorityForCert(types.HostCA, cert); err != nil {
		h.log.DebugContext(h.c.Server.Context(), "Unable to find SSH host CA", "error", err)
		return false
	}
	return true
}

// GitForwardingPermit is a permit that specifies the parameters/constraints associated with
// an authorized git forwarding attempt.
// NOTE: this type and its related functionality will likely be moved to the 'decision' family of
// packages in the future.
type GitForwardingPermit struct {
	// ClientIdleTimeout is the maximum amount of time the client is allowed to be idle.
	ClientIdleTimeout     time.Duration
	LockingMode           constants.LockingMode
	LockTargets           []types.LockTarget
	DisconnectExpiredCert time.Time
}

// loginChecker checks if the Teleport user should be able to login to
// a target.
type loginChecker interface {
	// evaluateSSHAccess checks the given certificate (supplied by a connected
	// client) to see if this certificate can be allowed to login as user:login
	// pair to requested server and if RBAC rules allow login.
	evaluateSSHAccess(ident *sshca.Identity, ca types.CertAuthority, clusterName string, target types.Server, osUser string) (*decisionpb.SSHAccessPermit, error)
}

type proxyingChecker interface {
	// evaluateProxying evaluates the capabilities/constraints related to a user's
	// attempt to access proxy forwarding.
	evaluateProxying(ident *sshca.Identity, ca types.CertAuthority, clusterName string) (*proxyingPermit, error)
}

type gitForwardingChecker interface {
	// evaluateGitForwarding evaluates the capabilities/constraints related to a user's
	// attempt to access git forwarding.
	evaluateGitForwarding(ident *sshca.Identity, ca types.CertAuthority, clusterName string, target types.Server) (*GitForwardingPermit, error)
}

type ahLoginChecker struct {
	log *slog.Logger
	c   *AuthHandlerConfig
}

type proxyingPermit struct {
	ClientIdleTimeout     time.Duration
	LockingMode           constants.LockingMode
	PrivateKeyPolicy      keys.PrivateKeyPolicy
	LockTargets           []types.LockTarget
	MaxConnections        int64
	DisconnectExpiredCert time.Time
	MappedRoles           []string
}

func (a *ahLoginChecker) evaluateProxying(ident *sshca.Identity, ca types.CertAuthority, clusterName string) (*proxyingPermit, error) {
	// Use the server's shutdown context.
	ctx := a.c.Server.Context()

	a.log.DebugContext(ctx, "evaluating ssh proxying attempt", "teleport_user", ident.Username)

	accessInfo, err := fetchAccessInfo(ident, ca, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName, a.c.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	netConfig, err := a.c.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load auth preference (used during calculation of locking mode)
	authPref, err := a.c.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKeyPolicy, err := accessChecker.PrivateKeyPolicy(authPref.GetPrivateKeyPolicy())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lockTargets := services.ProxyingLockTargets(clusterName, a.c.Server.HostUUID() /* id of underlying proxy */, accessInfo, ident)

	return &proxyingPermit{
		ClientIdleTimeout:     accessChecker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		LockingMode:           accessChecker.LockingMode(authPref.GetLockingMode()),
		PrivateKeyPolicy:      privateKeyPolicy,
		LockTargets:           lockTargets,
		MaxConnections:        accessChecker.MaxConnections(),
		DisconnectExpiredCert: getDisconnectExpiredCertFromSSHIdentity(accessChecker, authPref, ident),
		MappedRoles:           accessInfo.Roles,
	}, nil
}

func (a *ahLoginChecker) evaluateGitForwarding(ident *sshca.Identity, ca types.CertAuthority, clusterName string, target types.Server) (*GitForwardingPermit, error) {
	// Use the server's shutdown context.
	ctx := a.c.Server.Context()

	if clusterName != ca.GetClusterName() {
		// we don't currently support cross-cluster git forwarding (see comments in UserKeyAuth for details).
		return nil, trace.BadParameter("evaluateGitForwarding called with non-local identity (this is a bug)")
	}

	a.log.DebugContext(ctx, "checking git forwarding permissions", "teleport_user", ident.Username)

	accessInfo, err := fetchAccessInfo(ident, ca, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName, a.c.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	state, err := services.AccessStateFromSSHIdentity(ctx, ident, accessChecker, a.c.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := accessChecker.CheckAccess(target, state); err != nil {
		return nil, trace.Wrap(err)
	}

	netConfig, err := a.c.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load auth preference (used during calculation of locking mode)
	authPref, err := a.c.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lockTargets := services.GitForwardingLockTargets(clusterName, a.c.Server.HostUUID() /* id of git forwarder, not the target */, accessInfo, ident)

	return &GitForwardingPermit{
		ClientIdleTimeout:     accessChecker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		LockingMode:           accessChecker.LockingMode(authPref.GetLockingMode()),
		LockTargets:           lockTargets,
		DisconnectExpiredCert: getDisconnectExpiredCertFromSSHIdentity(accessChecker, authPref, ident),
	}, nil
}

// evaluateSSHAccess checks the given certificate (supplied by a connected
// client) to see if this certificate can be allowed to login as user:login
// pair to requested server and if RBAC rules allow login.
func (a *ahLoginChecker) evaluateSSHAccess(ident *sshca.Identity, ca types.CertAuthority, clusterName string, target types.Server, osUser string) (*decisionpb.SSHAccessPermit, error) {
	// Use the server's shutdown context.
	ctx := a.c.Server.Context()

	a.log.DebugContext(ctx, "checking permissions to login to node with RBAC checks", "teleport_user", ident.Username, "os_user", osUser)

	// get roles assigned to this user
	accessInfo, err := fetchAccessInfo(ident, ca, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName, a.c.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	state, err := services.AccessStateFromSSHIdentity(ctx, ident, accessChecker, a.c.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var isModeratedSessionJoin bool
	// custom moderated session join permissions allow bypass of the standard node access checks
	if osUser == teleport.SSHSessionJoinPrincipal &&
		auth.RoleSupportsModeratedSessions(accessChecker.Roles()) {

		// bypass of standard node access checks can only proceed if MFA is not required and/or
		// the MFA ceremony was already completed.
		if state.MFARequired == services.MFARequiredNever || state.MFAVerified {
			isModeratedSessionJoin = true
		}
	}

	if !isModeratedSessionJoin {
		// perform the primary node access check in all cases except for moderated session join
		if err := accessChecker.CheckAccess(
			target,
			state,
			services.NewLoginMatcher(osUser),
		); err != nil {
			return nil, trace.AccessDenied("user %s@%s is not authorized to login as %v@%s: %v",
				ident.Username, ca.GetClusterName(), osUser, clusterName, err)
		}
	}

	// load net config (used during calculation of client idle timeout)
	netConfig, err := a.c.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load auth preference (used during calculation of locking mode)
	authPref, err := a.c.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKeyPolicy, err := accessChecker.PrivateKeyPolicy(authPref.GetPrivateKeyPolicy())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lockTargets := services.SSHAccessLockTargets(clusterName, target.GetName(), osUser, accessInfo, ident)

	hostSudoers, err := accessChecker.HostSudoers(target)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var bpfEvents []string
	for event := range accessChecker.EnhancedRecordingSet() {
		bpfEvents = append(bpfEvents, event)
	}

	hostUsersInfo, err := accessChecker.HostUsers(target)
	if err != nil {
		if !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}
		// the way host user creation permissions currently work, an "access denied" just indicates
		// that host user creation is disabled, and does not indicate that access should be disallowed.
		// for the purposes of the decision service, we represent this disabled state as nil.
		hostUsersInfo = nil
	}

	return &decisionpb.SSHAccessPermit{
		ForwardAgent:          accessChecker.CheckAgentForward(osUser) == nil,
		X11Forwarding:         accessChecker.PermitX11Forwarding(),
		MaxConnections:        accessChecker.MaxConnections(),
		MaxSessions:           accessChecker.MaxSessions(),
		SshFileCopy:           accessChecker.CanCopyFiles(),
		PortForwardMode:       accessChecker.SSHPortForwardMode(),
		ClientIdleTimeout:     durationpb.New(accessChecker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout())),
		DisconnectExpiredCert: timestampFromGoTime(getDisconnectExpiredCertFromSSHIdentity(accessChecker, authPref, ident)),
		SessionRecordingMode:  string(accessChecker.SessionRecordingMode(constants.SessionRecordingServiceSSH)),
		LockingMode:           string(accessChecker.LockingMode(authPref.GetLockingMode())),
		PrivateKeyPolicy:      string(privateKeyPolicy),
		LockTargets:           decision.LockTargetsToProto(lockTargets),
		MappedRoles:           accessInfo.Roles,
		HostSudoers:           hostSudoers,
		BpfEvents:             bpfEvents,
		HostUsersInfo:         hostUsersInfo,
	}, nil
}

// fetchAccessInfo fetches the services.AccessChecker (after role mapping)
// together with the original roles (prior to role mapping) assigned to a
// Teleport user.
func fetchAccessInfo(ident *sshca.Identity, ca types.CertAuthority, clusterName string) (*services.AccessInfo, error) {
	var accessInfo *services.AccessInfo
	var err error
	if clusterName == ca.GetClusterName() {
		accessInfo = services.AccessInfoFromLocalSSHIdentity(ident)
	} else {
		accessInfo, err = services.AccessInfoFromRemoteSSHIdentity(ident, ca.CombinedMapping())
	}
	return accessInfo, trace.Wrap(err)
}

// authorityForCert checks if the certificate was signed by a Teleport
// Certificate Authority and returns it.
func (h *AuthHandlers) authorityForCert(caType types.CertAuthType, key ssh.PublicKey) (types.CertAuthority, error) {
	// get all certificate authorities for given type
	cas, err := h.c.AccessPoint.GetCertAuthorities(h.c.Server.Context(), caType, false)
	if err != nil {
		h.log.WarnContext(h.c.Server.Context(), "failed retrieving cert authority", "error", err)
		return nil, trace.Wrap(err)
	}

	// find the one that signed our certificate
	var ca types.CertAuthority
	for i := range cas {
		checkers, err := sshutils.GetCheckers(cas[i])
		if err != nil {
			h.log.WarnContext(h.c.Server.Context(), "unable to get cert checker for ca", "ca", cas[i].GetName(), "error", err)
			return nil, trace.Wrap(err)
		}
		for _, checker := range checkers {
			// if we have a certificate, compare the certificate signing key against
			// the ca key. otherwise check the public key that was passed in. this is
			// due to the differences in how this function is called by the user and
			// host checkers.
			switch v := key.(type) {
			case *ssh.Certificate:
				if apisshutils.KeysEqual(v.SignatureKey, checker) {
					ca = cas[i]
					break
				}
			default:
				if apisshutils.KeysEqual(key, checker) {
					ca = cas[i]
					break
				}
			}
		}
	}

	// the certificate was signed by unknown authority
	if ca == nil {
		return nil, trace.AccessDenied("the certificate signed by untrusted CA")
	}

	return ca, nil
}

func getDisconnectExpiredCertFromSSHIdentity(
	checker services.AccessChecker,
	authPref types.AuthPreference,
	identity *sshca.Identity,
) time.Time {
	// In the case where both disconnect_expired_cert and require_session_mfa are enabled,
	// the PreviousIdentityExpires value of the certificate will be used, which is the
	// expiry of the certificate used to issue the short lived MFA verified certificate.
	//
	// See https://github.com/gravitational/teleport/issues/18544

	// If the session doesn't need to be disconnected on cert expiry just return the default value.
	if !checker.AdjustDisconnectExpiredCert(authPref.GetDisconnectExpiredCert()) {
		return time.Time{}
	}

	if !identity.PreviousIdentityExpires.IsZero() {
		// If this is a short-lived mfa verified cert, return the certificate extension
		// that holds its' issuing cert's expiry value.
		return identity.PreviousIdentityExpires
	}

	// Otherwise just return the current cert's expiration
	return identity.GetValidBefore()
}

func timestampToGoTime(t *timestamppb.Timestamp) time.Time {
	// nil or "zero" Timestamps are mapped to Go's zero time (0-0-0 0:0.0) instead
	// of unix epoch. The latter avoids problems with tooling (eg, Terraform) that
	// sets structs to their defaults instead of using nil.
	if t.GetSeconds() == 0 && t.GetNanos() == 0 {
		return time.Time{}
	}
	return t.AsTime()
}

func timestampFromGoTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
