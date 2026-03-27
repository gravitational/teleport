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
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

const (
	// VersionPrefix is the prefix for the SSH client version string used by Teleport SSH clients.
	VersionPrefix = "SSH-2.0-Teleport"

	// DefaultClientVersion returns the default SSH client identification string used by Teleport SSH clients.
	DefaultClientVersion = VersionPrefix + "_" + api.Version

	// InBandMFAFeature is a flag included in the client version string to indicate support for in-band MFA (RFD 234).
	InBandMFAFeature = "mfav1"
)

// clientVersionWithFeatures returns a client version string that includes the specified features. If no features are
// provided, it returns the default client version string.
func clientVersionWithFeatures(features ...string) string {
	if len(features) == 0 {
		return DefaultClientVersion
	}

	return DefaultClientVersion + " " + strings.Join(features, ",")
}

// NonTeleportSSHVersionError is returned by ParseClientVersion when the provided SSH client version string does not
// have the expected Teleport prefix. The client is either not a Teleport client or is an older Teleport version that
// did not set a client version string.
type NonTeleportSSHVersionError struct{}

// Error returns the error message for NonTeleportSSHVersionError.
func (NonTeleportSSHVersionError) Error() string {
	return "SSH client version is not a Teleport version"
}

// ParseClientVersion parses the given SSH client version string and extracts the Teleport version and supported
// features. It returns the Teleport version and a slice of supported features. If no features are specified, the
// features slice will be nil. If the client version string does not have the expected Teleport prefix, it returns a
// NonTeleportSSHVersionError. If the client version string contains invalid characters, it returns a BadParameter
// error.
//
// It intentionally does not attempt to parse the Teleport version into a structured format, as the Teleport version
// string is primarily used for informational purposes and may include additional metadata in the future. It is also
// intentional that features may contain spaces.
//
//	Accepted formats are:
//	 - SSH-2.0-Teleport
//	 - SSH-2.0-Teleport <feature1,feature2,...>
//	 - SSH-2.0-Teleport_<teleport_version>
//	 - SSH-2.0-Teleport_<teleport_version> <feature1,feature2,...>
//	 - SSH-2.0-Teleport<teleport_version>
//	 - SSH-2.0-Teleport<teleport_version> <feature1,feature2,...>
func ParseClientVersion(clientVersion string) (string, []string, error) {
	// Ensure it is actually has the Teleport SSH client version prefix before doing any further parsing or validation.
	rest, ok := strings.CutPrefix(clientVersion, VersionPrefix)
	if !ok {
		return "", nil, NonTeleportSSHVersionError{}
	}

	// Sanity check that the client version string only contains valid ASCII characters we allow. Since this can be sent
	// by untrusted clients, we want to avoid any potential issues with invalid characters. We intentionally do not
	// return the specific invalid character in the error message since it could be used for malicious purposes.
	if strings.ContainsFunc(
		clientVersion,
		func(r rune) bool { return r < 32 || r > 126 },
	) {
		return "", nil, trace.BadParameter(
			"SSH client version contains invalid characters (only ASCII characters 32-126 are allowed)",
		)
	}

	// No version or features provided after the prefix, nothing to parse.
	if rest == "" {
		return "", nil, nil
	}

	// Separate the version part from the features part by the first space.
	versionPart, featuresPart, hasFeatures := strings.Cut(rest, " ")

	// Remove the leading underscore from the version part, if present.
	versionPart = strings.TrimPrefix(versionPart, "_")

	// If there are no features, return the version.
	if !hasFeatures {
		return versionPart, nil, nil
	}

	// Split the features part into individual features.
	features := strings.Split(featuresPart, ",")

	return versionPart, features, nil
}

// IsFeatureSupported checks if the given SSH client version string indicates support for the specified feature.
func IsFeatureSupported(clientVersion, feature string) (bool, error) {
	_, features, err := ParseClientVersion(clientVersion)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return slices.Contains(features, feature), nil
}

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

	case c.GetSigners != nil:
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

// SSHClientConfig builds a new [ssh.ClientConfig] from the wrapper config.
func (c ClientConfig) sshClientConfig() (*ssh.ClientConfig, error) {
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
		Config:            c.SSHConfig,
		User:              c.User,
		Auth:              authMethods,
		HostKeyCallback:   c.HostKeyCallback,
		BannerCallback:    c.BannerCallback,
		ClientVersion:     c.clientVersion(),
		HostKeyAlgorithms: slices.Clone(c.HostKeyAlgorithms),
		Timeout:           cmp.Or(c.Timeout, defaults.DefaultIOTimeout),
	}, nil
}

// Dial dials an SSH server using the SSH client config wrapper.
func Dial(
	ctx context.Context,
	network string,
	addr string,
	config ClientConfig,
	opts ...tracing.Option,
) (*tracessh.Client, error) {
	sshConfig, err := config.sshClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := tracessh.Dial(ctx, network, addr, sshConfig, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// NewClientWithTimeout creates a traced SSH client over an existing connection using the SSH client config wrapper.
func NewClientWithTimeout(
	ctx context.Context,
	conn net.Conn,
	addr string,
	config ClientConfig,
	opts ...tracing.Option,
) (*tracessh.Client, error) {
	sshConfig, err := config.sshClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := tracessh.NewClientWithTimeout(ctx, conn, addr, sshConfig, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

// NewClientConnWithTimeout creates a traced SSH client connection over an existing connection using the SSH client
// config wrapper.
func NewClientConnWithTimeout(
	ctx context.Context,
	conn net.Conn,
	addr string,
	config ClientConfig,
	opts ...tracing.Option,
) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	sshConfig, err := config.sshClientConfig()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	sshConn, chans, reqs, err := tracessh.NewClientConnWithTimeout(
		ctx,
		conn,
		addr,
		sshConfig,
		opts...,
	)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return sshConn, chans, reqs, nil
}

func (c ClientConfig) clientVersion() string {
	switch {
	// TODO(cthach): Set the in-band MFA feature flag using if AuthCallback is non-nil once
	// https://github.com/golang/go/issues/76146 is resolved and a new version of x/crypto/ssh is released.
	// case c.AuthCallback != nil:
	// 	return clientVersionWithFeatures(InBandMFAFeature)

	default:
		return DefaultClientVersion
	}
}

func (c ClientConfig) authMethods() ([]ssh.AuthMethod, error) {
	publicKeyAuthMethod, err := c.PublicKeyAuth.authMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []ssh.AuthMethod{publicKeyAuthMethod}, nil
}
