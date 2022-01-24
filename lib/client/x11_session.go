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

package client

import (
	"context"
	"os"
	"time"

	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// handleX11Forwarding handles X11 channel requests for the given server session.
// If X11 forwarding is not requested by the client, or it is rejected by the server,
// then X11 channel requests will be rejected.
func (ns *NodeSession) handleX11Forwarding(ctx context.Context, sess *ssh.Session) error {
	if !ns.nodeClient.TC.EnableX11Forwarding {
		return ns.rejectX11Channels(ctx)
	}

	display, err := x11.GetXDisplay()
	if err != nil {
		log.WithError(err).Info("X11 forwarding requested but $DISPLAY is invalid")
		return ns.rejectX11Channels(ctx)
	}

	if ns.nodeClient.TC.X11ForwardingTimeout != 0 {
		ns.x11RefuseTime = time.Now().Add(ns.nodeClient.TC.X11ForwardingTimeout)
	}

	if err := ns.setXAuthData(ctx, display); err != nil {
		return trace.Wrap(err)
	}

	// The client's xauth cookie should never be exposed to the server.
	// Instead, we create a spoof of the cookie to authenticate server
	// requests. During X11 forwarding, the spoofed cookie will be replaced
	// with the client's cookie to connect to the client's XServer.
	ns.spoofedXAuthEntry, err = ns.clientXAuthEntry.SpoofXAuthEntry()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := x11.RequestForwarding(sess, ns.spoofedXAuthEntry); err != nil {
		// If the X11 forwarding request fails, we must reject all X11 channel requests.
		return ns.rejectX11Channels(ctx)
	}

	// Start listening for new X11 channel requests from the server
	// and start X11 forwarding on those channels
	err = ns.serveX11Channels(ctx, sess)
	return trace.Wrap(err)
}

// setXAuthData generates new xauth data fro the client's local XServer.
// This will be used during X11 forwarding to forward and authorize
// XServer requests from the remote server to the client's XServer.
func (ns *NodeSession) setXAuthData(ctx context.Context, display x11.Display) error {
	if ns.nodeClient.TC.X11ForwardingTrusted {
		// For trusted X11 forwarding, we can create a random cookie without xauth
		// as it is only used to validate the server-client connection. Locally,
		// the client's XServer will ignore the cookie and use whatever authentication
		// mechanisms it would use as if the client made the request locally.
		log.Info("Creating a fake xauth token for trusted X11 forwarding.")
		log.Warn("Trusted X11 forwarding provides unmitigated access to your local XServer, use with caution")

		var err error
		ns.clientXAuthEntry, err = x11.NewFakeXAuthEntry(display)
		return trace.Wrap(err)
	}

	if err := x11.CheckXAuthPath(); err != nil {
		log.Info("trusted X11 forwarding requested but xauth is not installed")
		return trace.Wrap(err)
	}

	// Generate the xauth entry in a temporary file so it only exists within the context of this request.
	// The XServer will recognize the xauth data regardless of it's existence within the file system.
	xauthFile, err := os.CreateTemp("", "tsh-xauthfile-*")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := os.Remove(xauthFile.Name()); err != nil {
			log.WithError(err).Debug("Failed to remove temporary xauth file")
		}
	}()

	log.Info("creating an untrusted xauth cookie for untrusted X11 forwarding")
	cmd := x11.NewXAuthCommand(ctx, xauthFile.Name())
	if err := cmd.GenerateUntrustedCookie(display, ns.nodeClient.TC.X11ForwardingTimeout); err != nil {
		return trace.Wrap(err)
	}

	ns.clientXAuthEntry, err = x11.NewXAuthCommand(ctx, xauthFile.Name()).ReadEntry(display)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// serveX11Channels serves incoming X11 channels by starting X11 forwarding with the session.
func (ns *NodeSession) serveX11Channels(ctx context.Context, sess *ssh.Session) error {
	err := x11.ServeChannelRequests(ctx, ns.nodeClient.Client, func(ctx context.Context, nch ssh.NewChannel) {
		if !ns.x11RefuseTime.IsZero() && time.Now().After(ns.x11RefuseTime) {
			nch.Reject(ssh.Prohibited, "rejected X11 channel request after ForwardX11Timeout")
			log.Warn("rejected X11 forwarding attempt after the ForwardX11Timeout")
			return
		}

		var req x11.ChannelRequestPayload
		if err := ssh.Unmarshal(nch.ExtraData(), &req); err != nil {
			nch.Reject(ssh.Prohibited, "invalid payload")
			log.WithError(err).Debug("rejected X11 channel request with invalid payload")
			return
		}

		log.Debugf("received X11 channel request from %s:%d", req.OriginatorAddress, req.OriginatorPort)
		xchan, sin, err := nch.Accept()
		if err != nil {
			log.WithError(err).Debug("failed to accept X11 channel request")
			return
		}
		defer xchan.Close()

		// Scan the XServer request from the X11 channel for an xauth packet. If the xauth packet
		// is present and contains the spoofed cookie, then the cookie will be replaced with the
		// client's xauth cookie. Otherwise, the request will be denied.
		authPacket, err := x11.ReadAndRewriteXAuthPacket(xchan, ns.spoofedXAuthEntry, ns.clientXAuthEntry)
		if trace.IsAccessDenied(err) {
			log.Error("X11 connection rejected due to wrong authentication")
			return
		} else if err != nil {
			log.WithError(err).Debug("Failed to read xauth packet from X11 channel request")
			return
		}

		// Dial a connection to the client's XServer.
		xconn, err := ns.clientXAuthEntry.Display.Dial()
		if err != nil {
			log.WithError(err).Debug("Failed to connect to client's display")
			return
		}
		defer xconn.Close()

		// send the processed X11 auth packet to the client's XServer connection.
		if _, err := xconn.Write(authPacket); err != nil {
			log.WithError(err).Debug("Failed to write xauth packet")
			return
		}

		// Forward ssh requests on the X11 channels until X11 forwarding is complete
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			err := sshutils.ForwardRequests(ctx, sin, sess)
			if err != nil {
				log.WithError(err).Debug("Failed to forward ssh request from server during X11 forwarding")
			}
		}()

		if err := x11.Forward(ctx, xconn, xchan); err != nil {
			log.WithError(err).Debug("Encountered error during X11 forwarding")
		}
	})
	return trace.Wrap(err)
}

// rejectX11Channels rejects any incomign X11 channels for this node session.
func (ns *NodeSession) rejectX11Channels(ctx context.Context) error {
	err := x11.ServeChannelRequests(ctx, ns.nodeClient.Client, func(_ context.Context, nch ssh.NewChannel) {
		// According to RFC 4254, client "implementations MUST reject any X11 channel
		// open requests if they have not requested X11 forwarding. Following openssh's
		// example, we treat such a request as a break in attempt and warn the user.
		log.Warn("server tried X11 forwarding without client requesting it, this is likely a break-in attempt by a malicious user")
		nch.Reject(ssh.Prohibited, "")
	})
	return trace.Wrap(err)
}
