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

package common

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// accessDedupTTL is how long one app.access emission suppresses another for the same app session.
const accessDedupTTL = time.Hour

// AppForwardedIdentityEnvVar turns on RFD 0341's route-free App path, in both
// the Proxy and the Application Service. It is temporary, and goes with the
// path's gates when the feature ships.
const AppForwardedIdentityEnvVar = "TELEPORT_UNSTABLE_APP_FORWARDED_IDENTITY"

// AppAccessIdentitySource is the app.access identity_source: where a client's identity came from.
type AppAccessIdentitySource string

const (
	// AppAccessIdentitySourceUserCert is the user certificate the client
	// connected with.
	AppAccessIdentitySourceUserCert AppAccessIdentitySource = "user_cert"
	// AppAccessIdentitySourceProxy is an identity a Proxy forwarded with each request.
	AppAccessIdentitySourceProxy AppAccessIdentitySource = "proxy"
)

// AccessRecorder emits app.access events for authorized application access. Events are deduplicated per app session
// so that app.access stays an activity signal rather than a record of every request or connection.
type AccessRecorder struct {
	serverID string
	emitter  apievents.Emitter
	emitted  *utils.FnCache
}

// NewAccessRecorder returns an AccessRecorder that attributes events to serverID. It removes expired app sessions from
// its deduplication cache until ctx is done.
func NewAccessRecorder(ctx context.Context, clock clockwork.Clock, serverID string, emitter apievents.Emitter) (*AccessRecorder, error) {
	// Emission must outlive the request that triggered it, so the cache does not take ctx. Access that already passed
	// authorization is still recorded while the service drains.
	emitted, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:     accessDedupTTL,
		Clock:   clock,
		Context: context.Background(),
		// The audit emitter redelivers every event it accepts, so emission fails only when
		// its queue rejects the event, which retrying on the next access would not fix.
		ReloadOnErr: false,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r := &AccessRecorder{
		serverID: serverID,
		emitter:  emitter,
		emitted:  emitted,
	}
	go r.removeExpired(ctx, clock)
	return r, nil
}

// removeExpired removes expired app sessions from the cache until ctx is done. The cache only removes them when it is
// accessed, so an idle service would otherwise hold every session from an earlier burst of access.
func (r *AccessRecorder) removeExpired(ctx context.Context, clock clockwork.Clock) {
	// Removal scans every entry under the lock that each access also takes, so it runs far less often than access.
	// The interval only bounds how long expired entries linger, not when they expire.
	ticker := clock.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.Chan():
			r.emitted.RemoveExpired()
		case <-ctx.Done():
			return
		}
	}
}

// RecordAccess emits an app.access event for authorized access to app, unless emission was already attempted for the
// same app session within the deduplication period. A canceled ctx stops the caller waiting but not the emission.
func (r *AccessRecorder) RecordAccess(ctx context.Context, identity *tlsca.Identity, app types.Application, identitySource AppAccessIdentitySource) error {
	sessionID := identity.RouteToApp.SessionID
	if sessionID == "" {
		// This is unexpected. Every app certificate carries a session ID. If this condition triggers, something has
		// gone badly wrong. Bail early to avoid flooding logs.
		return trace.BadParameter("identity is missing an app session ID")
	}
	_, err := r.emitted.Get(ctx, sessionID, func(emitCtx context.Context) (struct{}, error) {
		// The event is built from the request context because the cache runs this on its own context, which does not
		// carry the client source address.
		return struct{}{}, trace.Wrap(r.emitter.EmitAuditEvent(emitCtx, r.newEvent(ctx, identity, app, identitySource)))
	})
	return trace.Wrap(err)
}

func (r *AccessRecorder) newEvent(ctx context.Context, identity *tlsca.Identity, app types.Application, identitySource AppAccessIdentitySource) *apievents.AppAccess {
	var remoteAddr string
	if addr, err := authz.ClientSrcAddrFromContext(ctx); err == nil {
		remoteAddr = addr.String()
	}
	return &apievents.AppAccess{
		Metadata: apievents.Metadata{
			Type:        events.AppAccessEvent,
			Code:        events.AppAccessCode,
			ClusterName: identity.RouteToApp.ClusterName,
		},
		UserMetadata: identity.GetUserMetadata(),
		// The session ID is left unset so that repeated events for one app session get distinct audit backend keys.
		SessionMetadata: apievents.SessionMetadata{
			WithMFA:          identity.MFAVerified,
			PrivateKeyPolicy: string(identity.PrivateKeyPolicy),
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerNamespace: apidefaults.Namespace,
			ServerID:        r.serverID,
			ServerVersion:   teleport.Version,
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: remoteAddr,
		},
		AppMetadata: apievents.AppMetadata{
			AppURI:        app.GetURI(),
			AppPublicAddr: app.GetPublicAddr(),
			AppName:       app.GetName(),
			AppTargetPort: uint32(identity.RouteToApp.TargetPort),
		},
		AppSessionId:   identity.RouteToApp.SessionID,
		IdentitySource: string(identitySource),
	}
}
