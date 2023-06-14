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

package reversetunnel

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/proxy"
)

// agentDialer dials an ssh server on behalf of an agent.
type agentDialer struct {
	client      auth.AccessCache
	username    string
	authMethods []ssh.AuthMethod
	fips        bool
	options     []proxy.DialerOptionFunc
	log         logrus.FieldLogger
	isClaimed   func(principals ...string) bool
}

// DialContext creates an ssh connection to the given address.
func (d *agentDialer) DialContext(ctx context.Context, addr utils.NetAddr) (SSHClient, error) {
	// Create a dialer (that respects HTTP proxies) and connect to remote host.
	dialer := proxy.DialerFromEnvironment(addr.Addr, d.options...)
	pconn, err := dialer.DialTimeout(ctx, addr.AddrNetwork, addr.Addr, apidefaults.DefaultIOTimeout)
	if err != nil {
		d.log.WithError(err).Debugf("Failed to dial %s.", addr.Addr)
		return nil, trace.Wrap(err)
	}

	var principals []string
	callback, err := apisshutils.NewHostKeyCallback(
		apisshutils.HostKeyCallbackConfig{
			GetHostCheckers: d.hostCheckerFunc(ctx),
			OnCheckCert: func(c *ssh.Certificate) error {
				if d.isClaimed != nil && d.isClaimed(c.ValidPrincipals...) {
					d.log.Debugf("Aborting SSH handshake because the proxy %q is already claimed by some other agent.", c.ValidPrincipals[0])
					return trace.Errorf("proxy already claimed")
				}

				principals = c.ValidPrincipals
				return nil
			},
			FIPS: d.fips,
		})
	if err != nil {
		d.log.Debugf("Failed to create host key callback for %v: %v.", addr.Addr, err)
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
		d.log.WithError(err).Debugf("Failed to create client to %v.", addr.Addr)
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
