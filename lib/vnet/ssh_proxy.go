// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"errors"
	"fmt"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/utils"
)

// proxySSHConnection transparently proxies incoming SSH channels and requests
// to a target SSH client.
func proxySSHConnection(
	ctx context.Context,
	targetClient *ssh.Client,
	channels <-chan ssh.NewChannel,
	requests <-chan *ssh.Request,
) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return proxyChannels(ctx, targetClient, channels)
	})
	g.Go(func() error {
		return proxyGlobalRequests(ctx, targetClient, requests)
	})
	return trace.Wrap(g.Wait(), "proxying SSH connection")
}

func proxyGlobalRequests(
	ctx context.Context,
	targetClient *ssh.Client,
	requests <-chan *ssh.Request,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case req, ok := <-requests:
			if !ok {
				return nil
			}
			ok, reply, err := targetClient.SendRequest(req.Type, req.WantReply, req.Payload)
			if err != nil {
				if !req.WantReply {
					// No reply was expected, log the error and continue
					// handling global requests.
					log.WarnContext(ctx, "Failed to forward global SSH request to target", "error", err)
					continue
				}
				err = trace.Wrap(err, "forwarding global request to target")
				if replyErr := req.Reply(false, nil); replyErr != nil {
					// A reply was expected and we failed to send one, we're in a
					// bad state, return an error to terminate the connection.
					return trace.NewAggregate(err, trace.Wrap(replyErr, "replying to request with error"))
				}
				// We failed to send the request, but we informed the client of
				// the failure with req.Reply(false, nil), so it's safe to
				// continue.
				continue
			}
			if err := req.Reply(ok, reply); err != nil {
				// A reply was expected by the client and returned by the target
				// but we failed to forward it back. We're in a bad state,
				// return an error to terminate the connection.
				return trace.Wrap(err, "forwarding reply from target back to client")
			}
		}
	}
}

func proxyChannels(
	ctx context.Context,
	targetClient *ssh.Client,
	channels <-chan ssh.NewChannel,
) (err error) {
	// Every channel will be handled in its own goroutine, but a recoverable
	// error from a single channel should not terminate other channels, so
	// handleNewChannel must avoid returning errors that should not terminate
	// the connection.
	g, ctx := errgroup.WithContext(ctx)
	defer func() {
		chanErr := g.Wait()
		if chanErr == nil ||
			utils.IsOKNetworkError(chanErr) ||
			errors.Is(chanErr, context.Canceled) {
			return
		}
		err = trace.NewAggregate(err, chanErr)
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case newChan, ok := <-channels:
			if !ok {
				return nil
			}
			g.Go(func() error {
				return trace.Wrap(handleNewChannel(ctx, targetClient, newChan),
					"unexpected error while handling SSH channel")
			})
		}
	}
}

func handleNewChannel(ctx context.Context, targetClient *ssh.Client, newChan ssh.NewChannel) error {
	log.DebugContext(ctx, "Handling new SSH channel", "type", newChan.ChannelType())

	// Try to open a corresponding channel on the target. If the target rejects
	// the channel, reject the incoming channel request.
	targetChan, targetChanRequests, err := targetClient.OpenChannel(newChan.ChannelType(), newChan.ExtraData())
	if err != nil {
		reason, message, unexpectedOpenChannelErr := rejectChannelReason(err)
		rejectErr := trace.Wrap(newChan.Reject(reason, message),
			"rejecting incoming channel request after failing to open channel on target")
		if unexpectedOpenChannelErr != nil || rejectErr != nil {
			// Either of these errors is unexpected so return an error
			// to terminate the full SSH connection.
			return trace.NewAggregate(unexpectedOpenChannelErr, rejectErr)
		}
		// The channel was successfully rejected, this is fine, the client may
		// have requested a channel type the target doesn't support.
		return nil
	}

	// Now that the target accepted the channel, accept the incoming channel
	// request.
	serverChan, serverChanRequests, acceptErr := newChan.Accept()
	if acceptErr != nil {
		acceptErr = trace.Wrap(acceptErr,
			"accepting incoming channel request")
		closeTargetChanErr := trace.Wrap(targetChan.Close(),
			"closing target channel after failing to accept incoming channel request")
		// Failing to accept an incoming channel request that the target already
		// accepted should probably be fatal, so return an error to terminate
		// the full SSH connection.
		return trace.NewAggregate(acceptErr, closeTargetChanErr)
	}

	// Use separate goroutines to:
	// 1. Bidirectionally copy channel data.
	// 2. Proxy requests from the client to the target.
	// 3. Proxy requests from the target to the client.
	// If any of these returns an error it will terminate this channel only.
	// No error will be returned from handleNewChannel because the error is
	// specific to this channel and should not terminate the full SSH
	// connection.
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// ProxyConn will copy channel data bidirectionally and close both
		// channels before returning.
		err := utils.ProxyConn(ctx, serverChan, targetChan)
		return trace.Wrap(err, "proxying channel data")
	})
	g.Go(func() error {
		err := proxyChannelRequests(ctx, targetChan, serverChan, serverChanRequests)
		return trace.Wrap(err, "forwarding channel requests from server to target")
	})
	g.Go(func() error {
		err := proxyChannelRequests(ctx, serverChan, targetChan, targetChanRequests)
		return trace.Wrap(err, "forwarding channel requests from target to server")
	})
	if err := g.Wait(); err != nil &&
		!utils.IsOKNetworkError(err) && !errors.Is(err, context.Canceled) {
		log.DebugContext(ctx, "SSH channel closed", "error", err)
	}
	return nil
}

func rejectChannelReason(err error) (reason ssh.RejectionReason, message string, unexpectedErr error) {
	var openChannelErr *ssh.OpenChannelError
	if errors.As(err, &openChannelErr) {
		return openChannelErr.Reason, openChannelErr.Message, nil
	}
	unexpectedErr = fmt.Errorf("unexpected error opening SSH channel: %w", err)
	return ssh.ConnectionFailed, unexpectedErr.Error(), trace.Wrap(unexpectedErr)
}

func proxyChannelRequests(
	ctx context.Context,
	dst, src ssh.Channel,
	requests <-chan *ssh.Request,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case req, ok := <-requests:
			if !ok {
				return nil
			}
			ok, err := dst.SendRequest(req.Type, req.WantReply, req.Payload)
			if err != nil {
				if !req.WantReply {
					// No reply was expected, log the error and continue
					// handling channel requests.
					log.WarnContext(ctx, "Failed to forward channel SSH request", "error", err)
					continue
				}
				err = trace.Wrap(err, "forwarding channel request")
				if replyErr := req.Reply(false, nil); replyErr != nil {
					// A reply was expected but we failed to send one, we're in
					// a bad state, return an error to terminate the channel.
					return trace.NewAggregate(err, trace.Wrap(replyErr, "replying to channel request with error"))
				}
				// We failed to forward the request, but we informed the client of
				// the failure with req.Reply(false, nil), so it's safe to
				// continue.
				continue
			}
			if err := req.Reply(ok, nil); err != nil {
				// A reply was expected, and we got one but failed to forward
				// it back. We're in a bad state, return an error to terminate
				// the channel.
				return trace.Wrap(err, "forwarding reply to channel request")
			}
		}
	}
}
