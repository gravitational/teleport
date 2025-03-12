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

package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
)

// handleX11Forwarding handles X11 channel requests for the given server session.
// If X11 forwarding is not requested by the client, or it is rejected by the server,
// then X11 channel requests will be rejected.
func (ns *NodeSession) handleX11Forwarding(ctx context.Context, sess *tracessh.Session) error {
	if !ns.nodeClient.TC.EnableX11Forwarding {
		return ns.rejectX11Channels(ctx)
	}

	display, err := x11.GetXDisplay()
	if err != nil {
		slog.InfoContext(ctx, "X11 forwarding requested but $DISPLAY is invalid", "err", err)
		return ns.rejectX11Channels(ctx)
	}

	if err := ns.setXAuthData(ctx, display); err != nil {
		return trace.Wrap(err)
	}

	// The client's xauth cookie should never be exposed to the server, so we
	// create a spoof of the cookie to send to the server for authentication.
	// During X11 forwarding, the spoofed cookie will be replaced
	// with the client's cookie to connect to the client's XServer.
	ns.spoofedXAuthEntry, err = ns.clientXAuthEntry.SpoofXAuthEntry()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := x11.RequestForwarding(sess.Session, ns.spoofedXAuthEntry); err != nil {
		// Notify the user that x11 forwarding request failed regardless of debug level
		fmt.Fprintln(os.Stderr, "X11 forwarding request failed")
		slog.DebugContext(ctx, "X11 forwarding request error", "err", err)
		// If the X11 forwarding request fails, we must reject all X11 channel requests.
		return ns.rejectX11Channels(ctx)
	}

	// Start listening for new X11 channel requests from the server
	// and start X11 forwarding on those channels
	err = ns.serveX11Channels(ctx, sess)
	return trace.Wrap(err)
}

// setXAuthData generates new xauth data for the client's local XServer.
// This will be used during X11 forwarding to forward and authorize
// XServer requests from the remote server to the client's XServer.
func (ns *NodeSession) setXAuthData(ctx context.Context, display x11.Display) error {
	checkXauthErr := x11.CheckXAuthPath()
	if checkXauthErr != nil && !ns.nodeClient.TC.X11ForwardingTrusted {
		slog.WarnContext(ctx, "Untrusted X11 forwarding requires xauth to be installed in order generated xauth key data")
		return trace.Wrap(checkXauthErr)
	}

	if ns.nodeClient.TC.X11ForwardingTrusted {
		slog.WarnContext(ctx, "Trusted X11 forwarding provides unmitigated access to your local XServer, use with caution")

		// Check for existing xauth data. If found, it must be used in order to connect to the client's XServer.
		var err error
		if checkXauthErr == nil {
			ns.clientXAuthEntry, err = x11.NewXAuthCommand(ctx, "" /* xauthFile */).ReadEntry(display)
			if !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}

		// If no xauth data was found, we can create a random cookie without xauth
		// as it is only used to validate the server-client connection. Locally,
		// the client's XServer will ignore the cookie and use whatever authentication
		// mechanisms it would use as if the client made the request locally.
		slog.InfoContext(ctx, "No xauth data found, creating a fake xauth cookie for trusted X11 forwarding.")

		ns.clientXAuthEntry, err = x11.NewFakeXAuthEntry(display)
		return trace.Wrap(err)
	}

	// Generate an untrusted xauth entry in a temporary file so it only exists within the context of this request.
	// The XServer will recognize the xauth data regardless of it's existence within the file system.
	xauthFile, err := os.CreateTemp("", "tsh-xauthfile-*")
	if err != nil {
		return trace.Wrap(err)
	}

	// Close the file so that xauth (in Windows) can successfully edit the file.
	// Otherwise, xauth will create a "<xauth>-n" new file and never transfers
	// the generated data into the actual "<xauth>" file.
	if err := xauthFile.Close(); err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := os.Remove(xauthFile.Name()); err != nil {
			slog.DebugContext(ctx, "Failed to remove temporary xauth file", "err", err)
		}
	}()

	// When an untrusted cookie expires, X requests with that cookie are not rejected, rather
	// the X Server ignores the unrecognized cookie and fail over to whatever authentication
	// mechanisms are in place. This is the same behavior used with the fake cookie used
	// above in trusted forwarding. Therefore it is essential that we deny any X requests made
	// after the cookie has expired, and so we set this timeout before generating the cookie.
	if ns.nodeClient.TC.X11ForwardingTimeout != 0 {
		ns.x11RefuseTime = time.Now().Add(ns.nodeClient.TC.X11ForwardingTimeout)
	}

	slog.InfoContext(ctx, "creating an untrusted xauth cookie for untrusted X11 forwarding", "xauth_file", xauthFile.Name())
	cmd := x11.NewXAuthCommand(ctx, xauthFile.Name())
	if err := cmd.GenerateUntrustedCookie(display, ns.nodeClient.TC.X11ForwardingTimeout); err != nil {
		return trace.Wrap(err)
	}

	ns.clientXAuthEntry, err = x11.NewXAuthCommand(ctx, xauthFile.Name()).ReadEntry(display)
	if err != nil {
		slog.ErrorContext(ctx, "untrusted X11 forwarding setup failed: failed to generate xauth key data")
		return trace.Wrap(err)
	}

	return nil
}

// serveX11Channels serves incoming X11 channels by starting X11 forwarding with the session.
func (ns *NodeSession) serveX11Channels(ctx context.Context, sess *tracessh.Session) error {
	err := x11.ServeChannelRequests(ctx, ns.nodeClient.Client.Client, func(ctx context.Context, nch ssh.NewChannel) {
		if !ns.x11RefuseTime.IsZero() && time.Now().After(ns.x11RefuseTime) {
			nch.Reject(ssh.Prohibited, "rejected X11 channel request after ForwardX11Timeout")
			slog.WarnContext(ctx, "rejected X11 forwarding attempt after the ForwardX11Timeout")
			return
		}

		var req x11.ChannelRequestPayload
		if err := ssh.Unmarshal(nch.ExtraData(), &req); err != nil {
			nch.Reject(ssh.Prohibited, "invalid payload")
			slog.DebugContext(ctx, "rejected X11 channel request with invalid payload", "err", err)
			return
		}

		slog.DebugContext(ctx, "received X11 channel request from %s:%d", req.OriginatorAddress, req.OriginatorPort)
		xchan, sin, err := nch.Accept()
		if err != nil {
			slog.DebugContext(ctx, "failed to accept X11 channel request", "err", err)
			return
		}
		defer xchan.Close()

		// Scan the XServer request from the X11 channel for an xauth packet. If the xauth packet
		// is present and contains the spoofed cookie, then the cookie will be replaced with the
		// client's xauth cookie. Otherwise, the request will be denied.
		authPacket, err := x11.ReadAndRewriteXAuthPacket(xchan, ns.spoofedXAuthEntry, ns.clientXAuthEntry)
		if trace.IsAccessDenied(err) {
			slog.ErrorContext(ctx, "X11 connection rejected due to wrong authentication", "err", err)
			return
		} else if err != nil {
			slog.DebugContext(ctx, "Failed to read xauth packet from X11 channel request", "err", err)
			return
		}

		// Dial a connection to the client's XServer.
		xconn, err := ns.clientXAuthEntry.Display.Dial()
		if err != nil {
			slog.DebugContext(ctx, "Failed to connect to client's display", "err", err)
			return
		}
		defer xconn.Close()

		// Send the processed X11 auth packet to the client's XServer connection.
		if _, err := xconn.Write(authPacket); err != nil {
			slog.DebugContext(ctx, "Failed to write xauth packet", "err", err)
			return
		}

		// Forward ssh requests on the X11 channels until X11 forwarding is complete
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			err := sshutils.ForwardRequests(ctx, sin, sess)
			if err != nil {
				slog.DebugContext(ctx, "Failed to forward ssh request from server during X11 forwarding", "err", err)
			}
		}()

		// Proxy the connection until the connection is closed or the request is canceled.
		if err := utils.ProxyConn(ctx, xconn, xchan); err != nil && !errors.Is(err, context.Canceled) {
			slog.DebugContext(ctx, "Encountered error during X11 forwarding", "err", err)
		}
	})
	return trace.Wrap(err)
}

// rejectX11Channels rejects any incomign X11 channels for this node session.
func (ns *NodeSession) rejectX11Channels(ctx context.Context) error {
	err := x11.ServeChannelRequests(ctx, ns.nodeClient.Client.Client, func(_ context.Context, nch ssh.NewChannel) {
		// According to RFC 4254, client "implementations MUST reject any X11 channel
		// open requests if they have not requested X11 forwarding". Following openssh's
		// example, we treat such a request as a break in attempt and warn the user.
		slog.WarnContext(ctx, "server tried X11 forwarding without client requesting it, this is likely a break-in attempt by a malicious user")
		nch.Reject(ssh.Prohibited, "")
	})
	return trace.Wrap(err)
}
