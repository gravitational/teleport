/*
Copyright 2021 Gravitational, Inc.

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
		Log:       h.log,
		SessionID: session.ID(sID),
		Streamer:  clt,
	})
	if err != nil {
		h.log.Errorf("couldn't create player for session %v: %v", sID, err)
		ws.WriteMessage(websocket.BinaryMessage,
			[]byte(`{"message": "error", "errorText": "Internal server error"}`))
		return nil, nil
	}

	defer player.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		desktop.ReceivePlaybackActions(h.log, ws, player)
	}()

	go func() {
		defer cancel()
		defer ws.Close()
		desktop.PlayRecording(ctx, h.log, ws, player)
	}()

	<-ctx.Done()
	return nil, nil
}
