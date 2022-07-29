/*
Copyright 2022 Gravitational, Inc.

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
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

type upgradeHandler func(ctx context.Context, conn net.Conn) error

func (h *Handler) selectConnectionUpgradeType(r *http.Request) (string, upgradeHandler, error) {
	// TODO use constants
	if r.Header.Get("Connection") != "Upgrade" {
		return "", nil, trace.BadParameter("not an upgrade request")
	}

	upgrades := r.Header.Values("Upgrade")
	for _, upgradeType := range upgrades {
		switch upgradeType {
		case "alpn":
			return upgradeType, h.upgradeToALPN, nil
		}
	}

	return "", nil, trace.BadParameter("unsupported upgrade types: %v", upgrades)
}

func (h *Handler) connectionUpgrade(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {

	upgradeType, handler, err := h.selectConnectionUpgradeType(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, trace.BadParameter("failed to hijack connection")
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	if err := writeUpgradeResponse(conn, upgradeType); err != nil {
		return nil, trace.Wrap(err)
	}

	conn.SetDeadline(time.Time{})
	return nil, trace.Wrap(handler(r.Context(), conn))
}

func (h *Handler) upgradeToALPN(ctx context.Context, conn net.Conn) error {
	if h.cfg.ALPNHandler == nil {
		return trace.BadParameter("missing ALPNHandler")
	}

	// TODO add pingconn

	err := h.cfg.ALPNHandler.HandleConnection(ctx, conn)
	if err != nil && !utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

func writeUpgradeResponse(w io.Writer, upgradeType string) error {
	header := make(http.Header)
	header.Add("Upgrade", upgradeType)
	response := &http.Response{
		Status:     http.StatusText(http.StatusSwitchingProtocols),
		StatusCode: http.StatusSwitchingProtocols,
		Header:     header,
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return response.Write(w)
}
