/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package service

import (
	"context"
	"log/slog"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	beamscommon "github.com/gravitational/teleport/lib/beams/common"
	"github.com/gravitational/teleport/lib/events"
	alpnproxy "github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// beamEgressHandlerConfig holds the dependencies for the beam egress ALPN handler.
type beamEgressHandlerConfig struct {
	clusterName string
	beamsClient beamsv1.BeamsServiceClient
	emitter     apievents.Emitter
	logger      *slog.Logger
}

// beamEgressHandler handles inbound teleport-beam-egress ALPN connections.
// tbot vnet uses this to proxy outbound TCP traffic from inside a beam to an
// allowed external domain.
//
// The client presents a cert with RouteToApp where:
//
//	SessionID = beam resource ID
//	URI       = beamegress://host:port
type beamEgressHandler struct {
	beamEgressHandlerConfig
	middleware *authz.Middleware
}

func newBeamEgressHandler(cfg beamEgressHandlerConfig) *beamEgressHandler {
	return &beamEgressHandler{
		beamEgressHandlerConfig: cfg,
		middleware: &authz.Middleware{
			ClusterName:   cfg.clusterName,
			AcceptedUsage: []string{teleport.UsageAppsOnly},
		},
	}
}

func (h *beamEgressHandler) handlerDecs() alpnproxy.HandlerDecs {
	return alpnproxy.HandlerDecs{
		MatchFunc: alpnproxy.MatchByProtocol(alpncommon.ProtocolBeamEgress),
		Handler:   h.handle,
	}
}

func (h *beamEgressHandler) handle(ctx context.Context, rawConn net.Conn) error {
	h.logger.InfoContext(ctx, "=== Beam egress handler invoked", "remote_addr", rawConn.RemoteAddr())

	tlsConn, ok := rawConn.(utils.TLSConn)
	if !ok {
		return trace.BadParameter("expected utils.TLSConn, got %T", rawConn)
	}

	ctx, err := h.middleware.WrapContextWithUser(ctx, tlsConn)
	if err != nil {
		h.logger.ErrorContext(ctx, "=== Failed to wrap context with user", "error", err)
		return trace.Wrap(err)
	}

	userIdentity, err := authz.UserFromContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsIdentity := userIdentity.GetIdentity()
	routeToApp := tlsIdentity.RouteToApp
	remoteAddr := rawConn.RemoteAddr().String()
	h.logger.InfoContext(ctx, "=== Beam egress request",
		"route_to_app_name", routeToApp.Name,
		"route_to_app_uri", routeToApp.URI,
		"route_to_app_session_id", routeToApp.SessionID,
		"route_to_app_public_addr", routeToApp.PublicAddr,
		"remote_addr", remoteAddr,
	)

	emitFailure := func(errMsg string) {
		if err := h.emitter.EmitAuditEvent(ctx, &apievents.AppSessionStart{
			Metadata: apievents.Metadata{
				Type:        events.AppSessionStartEvent,
				Code:        events.AppSessionStartFailureCode,
				ClusterName: h.clusterName,
			},
			UserMetadata: tlsIdentity.GetUserMetadata(),
			SessionMetadata: apievents.SessionMetadata{
				SessionID: routeToApp.SessionID,
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				RemoteAddr: remoteAddr,
			},
			AppMetadata: apievents.AppMetadata{
				AppURI:       "beamegress://" + routeToApp.PublicAddr,
				AppPublicAddr: routeToApp.PublicAddr,
			},
			Error: errMsg,
		}); err != nil {
			h.logger.WarnContext(ctx, "Failed to emit beam egress session start failure event", "error", err)
		}
	}

	// URI is not encoded in the TLS certificate, so we derive the target
	// host:port from PublicAddr which does survive the round-trip.
	hostPort := routeToApp.PublicAddr
	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		err := trace.Wrap(err, "invalid beam egress PublicAddr %q", hostPort)
		emitFailure(err.Error())
		return err
	}

	beam, err := h.beamsClient.GetBeam(ctx, &beamsv1.GetBeamRequest{
		Selector: &beamsv1.GetBeamRequest_Id{Id: routeToApp.Name},
	})
	if err != nil {
		err := trace.Wrap(err, "fetching beam %q", routeToApp.Name)
		emitFailure(err.Error())
		return err
	}

	// Verify the beam is owned by the user from the incoming cert.
	if beam.GetStatus().GetUser() != tlsIdentity.Username {
		err := trace.AccessDenied("beam %q is not owned by %q", routeToApp.Name, tlsIdentity.Username)
		emitFailure(err.Error())
		return err
	}

	h.logger.InfoContext(ctx, "=== Checking beam egress allowlist",
		"beam_name", beam.GetMetadata().GetName(),
		"allowed_domains", beam.GetSpec().GetAllowedDomains(),
		"host_port", hostPort,
	)

	if !beamscommon.EgressHostPortAllowed(beam.GetSpec().GetAllowedDomains(), hostPort) {
		err := trace.AccessDenied("domain %q is not in allowed domains for beam %q", hostPort, routeToApp.Name)
		h.logger.WarnContext(ctx, "=== Beam egress denied by allowlist", "error", err)
		emitFailure(err.Error())
		return err
	}

	h.logger.InfoContext(ctx, "=== Beam egress allowed, dialing target", "host_port", hostPort)

	if err := h.emitter.EmitAuditEvent(ctx, &apievents.AppSessionStart{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionStartEvent,
			Code:        events.AppSessionStartCode,
			ClusterName: h.clusterName,
		},
		UserMetadata: tlsIdentity.GetUserMetadata(),
		SessionMetadata: apievents.SessionMetadata{
			SessionID: routeToApp.SessionID,
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: remoteAddr,
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        "beamegress://" + hostPort,
			AppPublicAddr: hostPort,
			AppName:       beam.GetMetadata().GetName(),
		},
	}); err != nil {
		h.logger.WarnContext(ctx, "Failed to emit beam egress session start event", "error", err)
	}

	target, err := net.Dial("tcp", hostPort)
	if err != nil {
		h.logger.ErrorContext(ctx, "=== Failed to dial egress target", "host_port", hostPort, "error", err)
		return trace.Wrap(err, "dialing %q", hostPort)
	}
	h.logger.InfoContext(ctx, "=== Beam egress connection established, proxying", "host_port", hostPort, "local_addr", target.LocalAddr())
	return trace.Wrap(utils.ProxyConn(ctx, rawConn, target))
}
