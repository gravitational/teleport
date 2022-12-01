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
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"nhooyr.io/websocket"

	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/web/desktop"
)

func (h *Handler) desktopPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing sid in request URL")
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Safari doesn't handle compression correctly, so disable it
	// https://github.com/nhooyr/websocket/issues/218
	compressionMode := websocket.CompressionContextTakeover
	if strings.Contains(r.UserAgent(), "Safari") {
		compressionMode = websocket.CompressionDisabled
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: compressionMode,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer h.log.Debug("desktopPlaybackHandle websocket handler returned")
	desktop.NewPlayer(sID, ws, clt, h.log).Play(r.Context())
	return nil, nil
}
