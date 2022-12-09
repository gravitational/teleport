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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/web/playback"
)

func (h *Handler) playbackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
	createPlayer func(ws *websocket.Conn, sID string, clt auth.ClientI, log logrus.FieldLogger),
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing sid in request URL")
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	websocket.Handler(func(ws *websocket.Conn) {
		createPlayer(ws, sID, clt, h.log)
	}).ServeHTTP(w, r)
	return nil, nil
}

func (h *Handler) desktopPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) (interface{}, error) {
	return h.playbackHandle(w, r, p, ctx, site, func(ws *websocket.Conn, sID string, clt auth.ClientI, log logrus.FieldLogger) {
		defer h.log.Debug("desktopPlaybackHandle websocket handler returned")
		playback.NewDesktopPlayer(sID, ws, clt, h.log).Play(r.Context())
	})
}

func (h *Handler) ttyPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	ctx *SessionContext,
	site reversetunnel.RemoteSite,
) (interface{}, error) {
	return h.playbackHandle(w, r, p, ctx, site, func(ws *websocket.Conn, sID string, clt auth.ClientI, log logrus.FieldLogger) {
		defer h.log.Debug("ttyPlaybackHandle websocket handler returned")
		playback.NewTtyPlayer(sID, ws, clt, h.log).Play(r.Context())
	})
}
