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

package spanner

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// rpcInfo contains information about a remote procedure call.
type rpcInfo struct {
	// database is the name of the Spanner database within the instance for
	// which the RPC was called. This can be different for each RPC, overriding
	// the session database name as long as Teleport RBAC allows it.
	database string
	// procedure is the name of the remote procedure.
	procedure string
	// args is the RPC including all arguments.
	args *events.Struct
	// err contains an error if the RPC was rejected by Teleport.
	err error
}

func auditRPC(ctx context.Context, audit common.Audit, sessionCtx *common.Session, r rpcInfo) {
	audit.EmitEvent(ctx, makeRPCEvent(sessionCtx, r))
}

func makeRPCEvent(sessionCtx *common.Session, r rpcInfo) *events.SpannerRPC {
	sessionCtx = sessionCtx.WithDatabase(r.database)
	event := &events.SpannerRPC{
		Metadata: common.MakeEventMetadata(sessionCtx,
			libevents.DatabaseSessionSpannerRPCEvent,
			libevents.SpannerRPCCode),
		UserMetadata:     common.MakeUserMetadata(sessionCtx),
		SessionMetadata:  common.MakeSessionMetadata(sessionCtx),
		DatabaseMetadata: common.MakeDatabaseMetadata(sessionCtx),
		Status: events.Status{
			Success: true,
		},
		Procedure: r.procedure,
		Args:      r.args,
	}
	if r.err != nil {
		event.Metadata.Code = libevents.SpannerRPCDeniedCode
		event.Status = events.Status{
			Success:     false,
			Error:       trace.Unwrap(r.err).Error(),
			UserMessage: r.err.Error(),
		}
	}
	return event
}
