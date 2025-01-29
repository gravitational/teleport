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

package web

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/web/desktop"
)

func (h *Handler) desktopPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
	ws *websocket.Conn,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	player, err := player.New(&player.Config{
		Clock:     h.clock,
		Log:       h.logger,
		SessionID: session.ID(sID),
		Streamer:  clt,
		Context:   r.Context(),
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "couldn't create player for session", "session_id", sID, "error", err)
		ws.WriteMessage(websocket.BinaryMessage,
			[]byte(`{"message": "error", "errorText": "Internal server error"}`))
		return nil, nil
	}

	defer player.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		desktop.ReceivePlaybackActions(ctx, h.logger, ws, player)
	}()

	go func() {
		defer cancel()
		defer ws.Close()
		desktop.PlayRecording(ctx, h.logger, ws, player)
	}()

	<-ctx.Done()
	return nil, nil
}
