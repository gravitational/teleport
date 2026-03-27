// Copyright 2026 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"cmp"
	"context"
	"net"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

// PublicKeyAuthConfig configures the public-key authentication method used by a Teleport SSH client. Exactly one of
// Signers or GetSigners must be set.
type PublicKeyAuthConfig struct {
	// Signers contains static signers to use for public-key authentication.
	Signers []ssh.Signer

	// GetSigners dynamically returns signers for public-key authentication.
	GetSigners func() ([]ssh.Signer, error)
}

func (c PublicKeyAuthConfig) authMethod() (ssh.AuthMethod, error) {
	switch {
	case len(c.Signers) == 0 && c.GetSigners == nil:
		return nil, trace.BadParameter("public key auth requires Signers or GetSigners")

	case len(c.Signers) > 0 && c.GetSigners != nil:
		return nil, trace.BadParameter("public key auth supports exactly one of Signers or GetSigners")
	}

	if c.GetSigners != nil {
		return ssh.PublicKeysCallback(c.GetSigners), nil
	}

	// This shallow clones. May need to be improved if we have more complex signers or need thread safety.
	signers := slices.Clone(c.Signers)

	if slices.Contains(signers, nil) {
		return nil, trace.BadParameter("public key auth Signers must not contain nil entries")
	}

	return ssh.PublicKeys(signers...), nil
}

// ClientConfig configures a Teleport SSH client wrapper around tracessh.
type ClientConfig struct {
	// Config contains configuration data common to both ServerConfig and ClientConfig.
	SSHConfig ssh.Config

	// User contains the username to authenticate as.
	User string

	// PublicKeyAuth configures the required public-key authentication method.
	PublicKeyAuth PublicKeyAuthConfig

	// HostKeyCallback validates the server host key during the SSH handshake.
	HostKeyCallback ssh.HostKeyCallback

	// BannerCallback displays server banners during the SSH handshake.
	BannerCallback ssh.BannerCallback

	// HostKeyAlgorithms lists the accepted server host key algorithms in order of preference.
	HostKeyAlgorithms []string

	// Timeout is the maximum amount of time to wait for the underlying TCP connection to establish. If zero, Teleport's
	// default I/O timeout is used. Negative values are preserved.
	Timeout time.Duration

	// AuthCallback, if non-nil, is invoked before each authentication attempt.
	//
	// TODO(cthach): Enable when https://github.com/golang/go/issues/76146 is resolved and a new version of x/crypto/ssh
	// is released.
	// AuthCallback ssh.ClientAuthCallback
}

// SSHClientConfig builds a fresh [ssh.ClientConfig] from the wrapper config.
func (c ClientConfig) clientConfig() (*ssh.ClientConfig, error) {
	switch {
	case c.User == "":
		return nil, trace.BadParameter("config User must be set")

	case c.HostKeyCallback == nil:
		return nil, trace.BadParameter("config HostKeyCallback must be set")
	}

	authMethods, err := c.authMethods()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ssh.ClientConfig{
		Config:            c.SSHConfig, // TODO(cthach): Do we need to clone this? What about setting defaults? 🤔
		User:              c.User,
		Auth:              authMethods,
		HostKeyCallback:   c.HostKeyCallback,
		BannerCallback:    c.BannerCallback,
		ClientVersion:     c.clientVersion(),
		HostKeyAlgorithms: slices.Clone(c.HostKeyAlgorithms),
		Timeout:           cmp.Or(c.Timeout, defaults.DefaultIOTimeout),
	}, nil
}

// Dial dials an SSH server using a Teleport-specific SSH client config wrapper.
func Dial(
	ctx context.Context,
	network string,
	addr string,
	cfg ClientConfig,
	opts ...tracing.Option,
) (*tracessh.Client, error) {
	config, err := cfg.clientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := tracessh.Dial(ctx, network, addr, config, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// NewClientWithTimeout creates a traced SSH client over an existing connection using a Teleport-specific SSH client
// config wrapper.
func NewClientWithTimeout(
	ctx context.Context,
	conn net.Conn,
	addr string,
	cfg ClientConfig,
	opts ...tracing.Option,
) (*tracessh.Client, error) {
	config, err := cfg.clientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := tracessh.NewClientWithTimeout(ctx, conn, addr, config, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// NewClientConnWithTimeout creates a traced SSH client connection over an existing connection using a Teleport-specific
// SSH client config wrapper.
func NewClientConnWithTimeout(
	ctx context.Context,
	conn net.Conn,
	addr string,
	cfg ClientConfig,
	opts ...tracing.Option,
) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	config, err := cfg.clientConfig()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	sshConn, chans, reqs, err := tracessh.NewClientConnWithTimeout(
		ctx,
		conn,
		addr,
		config,
		opts...,
	)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return sshConn, chans, reqs, nil
}

func (c ClientConfig) clientVersion() string {
	// TODO(cthach): Set the in-band MFA feature flag using if AuthCallback is non-nil once
	// https://github.com/golang/go/issues/76146 is resolved and a new version of x/crypto/ssh is released.
	// if c.AuthCallback != nil {
	// 	return ClientVersionWithFeatures(InBandMFAFeature)
	// }

	return DefaultClientVersion
}

func (c ClientConfig) authMethods() ([]ssh.AuthMethod, error) {
	publicKeyAuthMethod, err := c.PublicKeyAuth.authMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []ssh.AuthMethod{publicKeyAuthMethod}, nil
}
