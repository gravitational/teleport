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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// selectConnectionUpgrade selects the requested upgrade type and returns the
// corresponding handler.
func (h *Handler) selectConnectionUpgrade(r *http.Request) (string, ConnectionHandler, error) {
	upgrades := r.Header.Values(constants.WebAPIConnUpgradeHeader)
	for _, upgradeType := range upgrades {
		switch upgradeType {
		case constants.WebAPIConnUpgradeTypeALPNPing:
			return upgradeType, h.upgradeALPNWithPing, nil
		case constants.WebAPIConnUpgradeTypeALPN:
			return upgradeType, h.upgradeALPN, nil
		}
	}

	return "", nil, trace.NotFound("unsupported upgrade types: %v", upgrades)
}

// connectionUpgrade handles connection upgrades.
func (h *Handler) connectionUpgrade(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	upgradeType, upgradeHandler, err := h.selectConnectionUpgrade(r)
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

	// Since w is hijacked, there is no point returning an error for response
	// starting at this point.
	if err := writeUpgradeResponse(conn, upgradeType); err != nil {
		h.log.WithError(err).Error("Failed to write upgrade response.")
		return nil, nil
	}

	if err := upgradeHandler(r.Context(), conn); err != nil && !utils.IsOKNetworkError(err) {
		h.log.WithError(err).Errorf("Failed to handle %v upgrade request.", upgradeType)
	}
	return nil, nil
}

func (h *Handler) upgradeALPN(ctx context.Context, conn net.Conn) error {
	if h.cfg.ALPNHandler == nil {
		return trace.BadParameter("missing ALPNHandler")
	}

	// ALPNHandler may handle some connections asynchronously. Here we want to
	// block until the handling is done by waiting until the connection is
	// closed.
	waitConn := newWaitConn(ctx, conn)
	defer waitConn.WaitForClose()

	return h.cfg.ALPNHandler(ctx, waitConn)
}

func (h *Handler) upgradeALPNWithPing(ctx context.Context, conn net.Conn) error {
	if h.cfg.ALPNHandler == nil {
		return trace.BadParameter("missing ALPNHandler")
	}

	pingConn := pingconn.New(conn)

	// Cancel ping background goroutine when connection is closed.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go h.startPing(ctx, pingConn)

	return h.upgradeALPN(ctx, pingConn)
}

func (h *Handler) startPing(ctx context.Context, pingConn *pingconn.PingConn) {
	ticker := time.NewTicker(defaults.ProxyPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := pingConn.WritePing()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					h.log.WithError(err).Warn("Failed to write ping message")
				}
				return
			}
		}
	}
}

func writeUpgradeResponse(w io.Writer, upgradeType string) error {
	header := make(http.Header)
	header.Add(constants.WebAPIConnUpgradeHeader, upgradeType)
	header.Add(constants.WebAPIConnUpgradeConnectionHeader, constants.WebAPIConnUpgradeConnectionType)
	response := &http.Response{
		Status:     http.StatusText(http.StatusSwitchingProtocols),
		StatusCode: http.StatusSwitchingProtocols,
		Header:     header,
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return response.Write(w)
}

// waitConn is a net.Conn that provides a "WaitForClose" function to wait until
// the connection is closed.
type waitConn struct {
	net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

// newWaitConn creates a new waitConn.
func newWaitConn(ctx context.Context, conn net.Conn) *waitConn {
	ctx, cancel := context.WithCancel(ctx)
	return &waitConn{
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// WaitForClose blocks until the Close() function of this connection is called.
func (conn *waitConn) WaitForClose() {
	<-conn.ctx.Done()
}

// Close implements net.Conn.
func (conn *waitConn) Close() error {
	err := conn.Conn.Close()
	conn.cancel()
	return trace.Wrap(err)
}
