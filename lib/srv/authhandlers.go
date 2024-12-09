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
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auditd"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/connectmycomputer"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
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
	ah.loginChecker = &ahLoginChecker{
		log: ah.log,
		c:   ah.c,
	}

	return ah, nil
}

// CreateIdentityContext returns an IdentityContext populated with information
// about the logged in user on the connection.
func (h *AuthHandlers) CreateIdentityContext(sconn *ssh.ServerConn) (IdentityContext, error) {
	identity := IdentityContext{
		TeleportUser: sconn.Permissions.Extensions[utils.CertTeleportUser],
		Login:        sconn.User(),
	}

	clusterName, err := h.c.AccessPoint.GetClusterName()
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	certRaw := []byte(sconn.Permissions.Extensions[utils.CertTeleportUserCertificate])
	certificate, err := apisshutils.ParseCertificate(certRaw)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}
	identity.Certificate = certificate
	identity.RouteToCluster = certificate.Extensions[teleport.CertExtensionTeleportRouteToCluster]
	if certificate.ValidBefore != 0 {
		identity.CertValidBefore = time.Unix(int64(certificate.ValidBefore), 0)
	}
	certAuthority, err := h.authorityForCert(types.UserCA, certificate.SignatureKey)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}
	identity.CertAuthority = certAuthority

	identity.UnmappedRoles, err = services.ExtractRolesFromCert(certificate)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	accessInfo, err := fetchAccessInfo(certificate, certAuthority, identity.TeleportUser, clusterName.GetClusterName())
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}
	identity.AllowedResourceIDs = accessInfo.AllowedResourceIDs
	identity.AccessChecker, err = services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), h.c.AccessPoint)
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}

	identity.Impersonator = certificate.Extensions[teleport.CertExtensionImpersonator]
	accessRequestIDs, err := ParseAccessRequestIDs(certificate.Extensions[teleport.CertExtensionTeleportActiveRequests])
	if err != nil {
		return IdentityContext{}, trace.Wrap(err)
	}
	identity.ActiveRequests = accessRequestIDs
	if _, ok := certificate.Extensions[teleport.CertExtensionDisallowReissue]; ok {
		identity.DisallowReissue = true
	}
	if _, ok := certificate.Extensions[teleport.CertExtensionRenewable]; ok {
		identity.Renewable = true
	}
	if botName, ok := certificate.Extensions[teleport.CertExtensionBotName]; ok {
		identity.BotName = botName
	}
	if botInstanceID, ok := certificate.Extensions[teleport.CertExtensionBotInstanceID]; ok {
		identity.BotInstanceID = botInstanceID
	}
	if generationStr, ok := certificate.Extensions[teleport.CertExtensionGeneration]; ok {
		generation, err := strconv.ParseUint(generationStr, 10, 64)
		if err != nil {
			return IdentityContext{}, trace.Wrap(err)
		}
		identity.Generation = generation
	}
	if allowedResourcesStr, ok := certificate.Extensions[teleport.CertExtensionAllowedResources]; ok {
		allowedResourceIDs, err := types.ResourceIDsFromString(allowedResourcesStr)
		if err != nil {
			return IdentityContext{}, trace.Wrap(err)
		}
		identity.AllowedResourceIDs = allowedResourceIDs
	}
	if previousIdentityExpires, ok := certificate.Extensions[teleport.CertExtensionPreviousIdentityExpires]; ok {
		asTime, err := time.Parse(time.RFC3339, previousIdentityExpires)
		if err != nil {
			return IdentityContext{}, trace.Wrap(err)
		}
		identity.PreviousIdentityExpires = asTime
	}

	return identity, nil
}

// CheckAgentForward checks if agent forwarding is allowed for the users RoleSet.
func (h *AuthHandlers) CheckAgentForward(ctx *ServerContext) error {
	if err := ctx.Identity.AccessChecker.CheckAgentForward(ctx.Identity.Login); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CheckX11Forward checks if X11 forwarding is permitted for the user's RoleSet.
func (h *AuthHandlers) CheckX11Forward(ctx *ServerContext) error {
	if !ctx.Identity.AccessChecker.PermitX11Forwarding() {
		return trace.AccessDenied("x11 forwarding not permitted")
	}
	return nil
}

func (h *AuthHandlers) CheckFileCopying(ctx *ServerContext) error {
	if !ctx.Identity.AccessChecker.CanCopyFiles() {
		return trace.Wrap(errRoleFileCopyingNotPermitted)
	}
	return nil
}

// CheckPortForward checks if port forwarding is allowed for the users RoleSet.
func (h *AuthHandlers) CheckPortForward(addr string, ctx *ServerContext) error {
	if ok := ctx.Identity.AccessChecker.CanPortForward(); !ok {
		systemErrorMessage := fmt.Sprintf("port forwarding not allowed by role set: %v", ctx.Identity.AccessChecker.RoleNames())
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

		return trace.AccessDenied(userErrorMessage)
	}

	return nil
}

// UserKeyAuth implements SSH client authentication using public keys and is
// called by the server every time the client connects.
func (h *AuthHandlers) UserKeyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
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
	teleportUser := cert.KeyId

	connectionDiagnosticID := cert.Extensions[teleport.CertExtensionConnectionDiagnosticID]

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
			connectMyComputerRoleName := connectmycomputer.GetRoleNameForUser(teleportUser)

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
				connectMyComputerRoleName := connectmycomputer.GetRoleNameForUser(teleportUser)
				nodeLabel := fmt.Sprintf("%s: %s", types.ConnectMyComputerNodeOwnerLabel, teleportUser)
				message = fmt.Sprintf(
					"You are not authorized to access this node. Ensure that you hold the role %q and that "+
						"no role denies you access to the login %q and to nodes labeled with %q.",
					connectMyComputerRoleName, principal, nodeLabel)
			}

			traceType = types.ConnectionDiagnosticTrace_RBAC_NODE
		}

		if err := h.maybeAppendDiagnosticTrace(ctx, connectionDiagnosticID,
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
				User:          teleportUser,
				TrustedDevice: eventDeviceMetadataFromCert(cert),
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
			TeleportUser: teleportUser,
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

	permissions, err := certChecker.Authenticate(conn, key)
	if err != nil {
		certificateMismatchCount.Inc()
		recordFailedLogin(err)
		return nil, trace.Wrap(err)
	}
	log.DebugContext(ctx, "Successfully authenticated")

	clusterName, err := h.c.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// this is the only way we know of to pass valid additional data about the
	// connection to the handlers
	permissions.Extensions[utils.CertTeleportUser] = teleportUser
	permissions.Extensions[utils.CertTeleportClusterName] = clusterName.GetClusterName()
	permissions.Extensions[utils.CertTeleportUserCertificate] = string(ssh.MarshalAuthorizedKey(cert))

	switch cert.CertType {
	case ssh.UserCert:
		permissions.Extensions[utils.ExtIntCertType] = utils.ExtIntCertTypeUser
	case ssh.HostCert:
		permissions.Extensions[utils.ExtIntCertType] = utils.ExtIntCertTypeHost
	default:
		log.WarnContext(ctx, "Received unexpected cert type", "cert_type", cert.CertType)
	}

	if h.isProxy() {
		return permissions, nil
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

	// check if the user has permission to log into the node.
	if h.c.Component == teleport.ComponentForwardingNode {
		// If we are forwarding the connection, the target node
		// exists and it is an agentless node, preform an RBAC check.
		// Otherwise if the target node does not exist the node is
		// probably an unregistered SSH node; do not preform an RBAC check
		if h.c.TargetServer != nil && h.c.TargetServer.IsOpenSSHNode() {
			err = h.canLoginWithRBAC(cert, ca, clusterName.GetClusterName(), h.c.TargetServer, teleportUser, conn.User())
		}
	} else {
		// the SSH server is a Teleport node, preform an RBAC check now
		err = h.canLoginWithRBAC(cert, ca, clusterName.GetClusterName(), h.c.Server.GetInfo(), teleportUser, conn.User())
	}
	if err != nil {
		log.ErrorContext(ctx, "Permission denied", "error", err)
		recordFailedLogin(err)
		return nil, trace.Wrap(err)
	}

	if err := h.maybeAppendDiagnosticTrace(ctx, connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_RBAC_NODE,
		"You have access to the Node.",
		nil,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.maybeAppendDiagnosticTrace(ctx, connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_CONNECTIVITY,
		"Node is alive and reachable.",
		nil,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := h.maybeAppendDiagnosticTrace(ctx, connectionDiagnosticID,
		types.ConnectionDiagnosticTrace_RBAC_PRINCIPAL,
		"The requested principal is allowed.",
		nil,
	); err != nil {
		return nil, trace.Wrap(err)
	}

	return permissions, nil
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

// loginChecker checks if the Teleport user should be able to login to
// a target.
type loginChecker interface {
	// canLoginWithRBAC checks the given certificate (supplied by a connected
	// client) to see if this certificate can be allowed to login as user:login
	// pair to requested server and if RBAC rules allow login.
	canLoginWithRBAC(cert *ssh.Certificate, ca types.CertAuthority, clusterName string, target types.Server, teleportUser, osUser string) error
}

type ahLoginChecker struct {
	log *slog.Logger
	c   *AuthHandlerConfig
}

// canLoginWithRBAC checks the given certificate (supplied by a connected
// client) to see if this certificate can be allowed to login as user:login
// pair to requested server and if RBAC rules allow login.
func (a *ahLoginChecker) canLoginWithRBAC(cert *ssh.Certificate, ca types.CertAuthority, clusterName string, target types.Server, teleportUser, osUser string) error {
	// Use the server's shutdown context.
	ctx := a.c.Server.Context()

	a.log.DebugContext(ctx, "Checking permissions for (%v,%v) to login to node with RBAC checks.", teleportUser, osUser)

	// get roles assigned to this user
	accessInfo, err := fetchAccessInfo(cert, ca, teleportUser, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	accessChecker, err := services.NewAccessChecker(accessInfo, clusterName, a.c.AccessPoint)
	if err != nil {
		return trace.Wrap(err)
	}

	authPref, err := a.c.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	state := accessChecker.GetAccessState(authPref)
	_, state.MFAVerified = cert.Extensions[teleport.CertExtensionMFAVerified]

	// Certain hardware-key based private key policies are treated as MFA verification.
	if policyString, ok := cert.Extensions[teleport.CertExtensionPrivateKeyPolicy]; ok {
		if keys.PrivateKeyPolicy(policyString).MFAVerified() {
			state.MFAVerified = true
		}
	}

	// we don't need to check the RBAC for the node if they are only allowed to join sessions
	if osUser == teleport.SSHSessionJoinPrincipal &&
		auth.RoleSupportsModeratedSessions(accessChecker.Roles()) {

		// allow joining if cluster wide MFA is not required
		if state.MFARequired == services.MFARequiredNever {
			return nil
		}

		// only allow joining if the MFA ceremony was completed
		// first if cluster wide MFA is enabled
		if state.MFAVerified {
			return nil
		}
	}

	state.EnableDeviceVerification = true
	state.DeviceVerified = dtauthz.IsSSHDeviceVerified(cert)

	// check if roles allow access to server
	if err := accessChecker.CheckAccess(
		target,
		state,
		services.NewLoginMatcher(osUser),
	); err != nil {
		return trace.AccessDenied("user %s@%s is not authorized to login as %v@%s: %v",
			teleportUser, ca.GetClusterName(), osUser, clusterName, err)
	}

	return nil
}

// fetchAccessInfo fetches the services.AccessChecker (after role mapping)
// together with the original roles (prior to role mapping) assigned to a
// Teleport user.
func fetchAccessInfo(cert *ssh.Certificate, ca types.CertAuthority, teleportUser string, clusterName string) (*services.AccessInfo, error) {
	var accessInfo *services.AccessInfo
	var err error
	if clusterName == ca.GetClusterName() {
		accessInfo, err = services.AccessInfoFromLocalCertificate(cert)
	} else {
		accessInfo, err = services.AccessInfoFromRemoteCertificate(cert, ca.CombinedMapping())
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

// isProxy returns true if it's a regular SSH proxy.
func (h *AuthHandlers) isProxy() bool {
	return h.c.Component == teleport.ComponentProxy
}

// AccessRequests are the access requests associated with a session
type AccessRequests struct {
	IDs []string `json:"access_requests"`
}

func ParseAccessRequestIDs(str string) ([]string, error) {
	var accessRequestIDs []string
	var ar AccessRequests

	if str == "" {
		return []string{}, nil
	}
	err := json.Unmarshal([]byte(str), &ar)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, v := range ar.IDs {
		id, err := uuid.Parse(v)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "failed to parse access request ID")
		}
		if fmt.Sprintf("%v", id) == "" {
			return nil, trace.Errorf("invalid uuid: %v", id)
		}
		accessRequestIDs = append(accessRequestIDs, v)
	}
	return accessRequestIDs, nil
}
