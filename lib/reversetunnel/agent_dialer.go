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

package reversetunnel

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"
)

const proxyAlreadyClaimedError = "proxy already claimed"

// isProxyAlreadyClaimed returns true if the error is non-nil and its message
// ends with "proxy already claimed" (we can't extract a better sentinel out of
// a SSH handshake, unfortunately).
func isProxyAlreadyClaimed(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasSuffix(err.Error(), proxyAlreadyClaimedError)
}

// agentDialer dials an ssh server on behalf of an agent.
type agentDialer struct {
	client      authclient.AccessCache
	username    string
	authMethods []ssh.AuthMethod
	fips        bool
	options     []proxy.DialerOptionFunc
	logger      *slog.Logger
	isClaimed   func(principals ...string) bool
}

// DialContext creates an ssh connection to the given address.
func (d *agentDialer) DialContext(ctx context.Context, addr utils.NetAddr) (SSHClient, error) {
	// Create a dialer (that respects HTTP proxies) and connect to remote host.
	dialer := proxy.DialerFromEnvironment(addr.Addr, d.options...)
	pconn, err := dialer.DialTimeout(ctx, addr.AddrNetwork, addr.Addr, apidefaults.DefaultIOTimeout)
	if err != nil {
		d.logger.DebugContext(ctx, "Failed to dial", "error", err, "target_addr", addr.Addr)
		return nil, trace.Wrap(err)
	}

	var principals []string
	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: d.hostCheckerFunc(ctx),
			OnCheckCert: func(c *ssh.Certificate) error {
				if d.isClaimed != nil && d.isClaimed(c.ValidPrincipals...) {
					d.logger.DebugContext(ctx, "Aborting SSH handshake because the proxy is already claimed by some other agent.", "proxy_id", c.ValidPrincipals[0])
					// the error message must end with
					// [proxyAlreadyClaimedError] to be recognized by
					// [isProxyAlreadyClaimed]
					return trace.Errorf(proxyAlreadyClaimedError)
				}

				principals = c.ValidPrincipals
				return nil
			},
			FIPS: d.fips,
		})
	if err != nil {
		d.logger.DebugContext(ctx, "Failed to create host key callback", "target_addr", addr.Addr, "error", err)
		return nil, trace.Wrap(err)
	}

	// Build a new client connection. This is done to get access to incoming
	// global requests which dialer.Dial would not provide.
	conn, chans, reqs, err := tracessh.NewClientConn(ctx, pconn, addr.Addr, &ssh.ClientConfig{
		User:            d.username,
		Auth:            d.authMethods,
		HostKeyCallback: callback,
		Timeout:         apidefaults.DefaultIOTimeout,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// ssh.NewClient will loop over the global requests channel in a goroutine,
	// rejecting all requests; we want to handle the global requests ourselves,
	// so we feed it a closed channel to have the goroutine exit immediately.
	emptyRequests := make(chan *ssh.Request)
	close(emptyRequests)

	client := tracessh.NewClient(conn, chans, emptyRequests)

	return &sshClient{
		Client:      client,
		requests:    reqs,
		newChannels: chans,
		principals:  principals,
	}, nil
}

// hostCheckerFunc wraps a apisshutils.CheckersGetter function with a context.
func (d *agentDialer) hostCheckerFunc(ctx context.Context) apisshutils.CheckersGetter {
	return func() ([]ssh.PublicKey, error) {
		cas, err := d.client.GetCertAuthorities(ctx, types.HostCA, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var keys []ssh.PublicKey
		for _, ca := range cas {
			checkers, err := sshutils.GetCheckers(ca)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			keys = append(keys, checkers...)
		}
		return keys, nil
	}
}

// sshClient implements the SSHClient interface.
type sshClient struct {
	*tracessh.Client
	requests    <-chan *ssh.Request
	newChannels <-chan ssh.NewChannel
	principals  []string
}

// NewChannels is a channel that receieves ssh new channel requests.
func (c *sshClient) NewChannels() <-chan ssh.NewChannel {
	return c.newChannels
}

// GlobalRequests is a channel that receives global ssh requests.
func (c *sshClient) GlobalRequests() <-chan *ssh.Request {
	return c.requests
}

// Principals is a list of principals for the underlying ssh connection.
func (c *sshClient) Principals() []string {
	return c.principals
}

// Reply handles replying to a request.
func (c *sshClient) Reply(request *ssh.Request, ok bool, payload []byte) error {
	return request.Reply(ok, payload)
}
