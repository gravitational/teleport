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

	// DefaultClientVersion is the default SSH client identification string used by Teleport SSH clients.
	//
	// The Teleport version included in the client version string is intended for informational/debugging purposes only
	// and should NEVER be relied upon for inferring client capabilities or behavior. The presence of specific features
	// should be determined by parsing the version string for the expected feature flags. The string is not guaranteed
	// to be in any specific format, so it should not be parsed as a structured version string (e.g., semantic
	// versioning). DO NOT USE the Teleport version for any logic other than display or logging.
	DefaultClientVersion = VersionPrefix + "_" + api.Version

	// InBandMFAFeature is a flag included in the client version string to indicate support for in-band MFA (RFD 234).
	InBandMFAFeature = "mfav1"
)

// ClientVersionWithFeatures returns a client version string that includes the specified features. If no features are
// provided, it returns the default client version string.
func ClientVersionWithFeatures(features ...string) string {
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
//	 - SSH-2.0-Teleport_<teleport_version>
//	 - SSH-2.0-Teleport_<teleport_version> <feature1,feature2,...>
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
		func(r rune) bool { return r < ' ' || r > '~' },
	) {
		return "", nil, trace.BadParameter(
			"SSH client version contains invalid characters (only ASCII characters 32-126 are allowed)",
		)
	}

	// The Teleport client name and the version are separated by an underscore. This is to ensure consistency with
	// OpenSSH client version strings, which also use an underscore to separate the client name from the version.
	if !strings.HasPrefix(rest, "_") {
		return "", nil, trace.BadParameter("SSH client name and version must be separated by an underscore")
	}

	// Remove the leading underscore.
	rest = strings.TrimPrefix(rest, "_")

	// Reject an empty version after the required underscore.
	if rest == "" {
		return "", nil, trace.BadParameter("SSH client version must include a non-empty Teleport version")
	}

	// Separate the version part from the features part by the first space.
	versionPart, featuresPart, hasFeatures := strings.Cut(rest, " ")

	if versionPart == "" {
		return "", nil, trace.BadParameter("SSH client version must include a non-empty Teleport version")
	}

	// If there are no features, return the version.
	if !hasFeatures || featuresPart == "" {
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

// PublicKeyAuthConfig configures the public-key authentication method used by a Teleport SSH client.
type PublicKeyAuthConfig struct {
	// Signers dynamically returns signers for public-key authentication.
	Signers func() ([]ssh.Signer, error)
}

// IsEmpty returns true if the PublicKeyAuthConfig does not have an effective Signers function set.
func (c PublicKeyAuthConfig) IsEmpty() bool {
	return c.Signers == nil
}

func (c PublicKeyAuthConfig) authMethod() (ssh.AuthMethod, error) {
	if c.IsEmpty() {
		return nil, trace.BadParameter("public key auth requires Signers")
	}

	return ssh.PublicKeysCallback(c.Signers), nil
}

// ClientConfig defines all client-side parameters required to establish and authenticate an SSH connection with
// Teleport. The minimal set of required parameters is User, HostKeyCallback, and PublicKeyAuth.Signers. The rest are
// optional parameters that can be used to customize the SSH connection behavior. The client version string sent during
// the SSH handshake is determined based on how the fields are set, with the default being DefaultClientVersion if no
// features are indicated.
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

// SSHClientConfig builds a new [ssh.ClientConfig] from the client config.
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

// IsEmpty() returns true if the config does not have any effective values.
func (c ClientConfig) IsEmpty() bool {
	return c.User == "" &&
		c.PublicKeyAuth.IsEmpty() &&
		c.HostKeyCallback == nil
}

// Client is a thin wrapper around tracessh.Client.
type Client = tracessh.Client

// Dial dials an SSH server using the client config.
func Dial(
	ctx context.Context,
	network string,
	addr string,
	config ClientConfig,
	opts ...tracing.Option,
) (*Client, error) {
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

// NewClient creates a traced SSH client over an existing connection using the client config.
//
// Timeout behavior is determined by [ClientConfig.Timeout]:
//   - If Timeout > 0, the SSH handshake respects the earlier of the context deadline or Timeout.
//   - If Timeout == 0, the SSH handshake uses Teleport's default I/O timeout.
//   - If Timeout < 0, the SSH handshake only respects the context deadline or cancellation.
func NewClient(
	ctx context.Context,
	conn net.Conn,
	addr string,
	config ClientConfig,
	opts ...tracing.Option,
) (*Client, error) {
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

// NewClientConn creates a traced SSH client connection over an existing connection using the client config.
//
// Timeout behavior is determined by [ClientConfig.Timeout]:
//   - If Timeout > 0, the SSH handshake respects the earlier of the context deadline or Timeout.
//   - If Timeout == 0, the SSH handshake uses Teleport's default I/O timeout.
//   - If Timeout < 0, the SSH handshake only respects the context deadline or cancellation.
func NewClientConn(
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
