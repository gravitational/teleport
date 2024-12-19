/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package git

import (
	"context"
	"net"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/sshutils"
)

// knownGithubDotComFingerprints contains a list of known GitHub fingerprints.
//
// https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints
//
// TODO(greedy52) these fingerprints can change (e.g. GitHub changed its RSA
// key in 2023 because of an incident). Instead of hard-coding the values, we
// should try to periodically (e.g. once per day) poll them from the API.
var knownGithubDotComFingerprints = []string{
	"SHA256:uNiVztksCsDhcc0u9e8BujQXVUpKZIDTMczCvj3tD2s",
	"SHA256:br9IjFspm1vxR3iA35FWE+4VTyz1hYVLIE2t1/CeyWQ",
	"SHA256:p2QAMXNIC1TJYWeIOttrVc98/R1BUFWu3/LiyKgUfQM",
	"SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU",
}

// VerifyGitHubHostKey is an ssh.HostKeyCallback that verifies the host key
// belongs to "github.com".
func VerifyGitHubHostKey(_ string, _ net.Addr, key ssh.PublicKey) error {
	actualFingerprint := ssh.FingerprintSHA256(key)
	if slices.Contains(knownGithubDotComFingerprints, actualFingerprint) {
		return nil
	}
	return trace.BadParameter("cannot verify github.com: unknown fingerprint %v algo %v", actualFingerprint, key.Type())
}

// AuthPreferenceGetter is an interface for retrieving the current configured
// cluster auth preference.
type AuthPreferenceGetter interface {
	// GetAuthPreference returns the current cluster auth preference.
	GetAuthPreference(context.Context) (types.AuthPreference, error)
}

// GitHubUserCertGenerator is an interface to generating user certs for
// connecting to GitHub.
type GitHubUserCertGenerator interface {
	// GenerateGitHubUserCert signs an SSH certificate for GitHub integration.
	GenerateGitHubUserCert(context.Context, *integrationv1.GenerateGitHubUserCertRequest, ...grpc.CallOption) (*integrationv1.GenerateGitHubUserCertResponse, error)
}

// GitHubSignerConfig is the config for MakeGitHubSigner.
type GitHubSignerConfig struct {
	// Server is the target Git server.
	Server types.Server
	// GitHubUserID is the ID of the GitHub user to impersonate.
	GitHubUserID string
	// TeleportUser is the Teleport username
	TeleportUser string
	// AuthPreferenceGetter is used to get auth preference.
	AuthPreferenceGetter AuthPreferenceGetter
	// GitHubUserCertGenerator generate
	GitHubUserCertGenerator GitHubUserCertGenerator
	// IdentityExpires is the time that the identity should expire.
	IdentityExpires time.Time
	// Clock is used to control time.
	Clock clockwork.Clock
}

func (c *GitHubSignerConfig) CheckAndSetDefaults() error {
	if c.Server == nil {
		return trace.BadParameter("missing target server")
	}
	if c.Server.GetGitHub() == nil {
		return trace.BadParameter("missing GitHub spec")
	}
	if c.GitHubUserID == "" {
		return trace.BadParameter("missing GitHubUserID")
	}
	if c.TeleportUser == "" {
		return trace.BadParameter("missing TeleportUser")
	}
	if c.AuthPreferenceGetter == nil {
		return trace.BadParameter("missing AuthPreferenceGetter")
	}
	if c.GitHubUserCertGenerator == nil {
		return trace.BadParameter("missing GitHubUserCertGenerator")
	}
	if c.IdentityExpires.IsZero() {
		return trace.BadParameter("missing IdentityExpires")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

func (c *GitHubSignerConfig) certTTL() time.Duration {
	userExpires := c.IdentityExpires.Sub(c.Clock.Now())
	if userExpires > defaultGitHubUserCertTTL {
		return defaultGitHubUserCertTTL
	}
	return userExpires
}

// MakeGitHubSigner generates an ssh.Signer that can impersonate a GitHub user
// to connect to GitHub.
func MakeGitHubSigner(ctx context.Context, config GitHubSignerConfig) (ssh.Signer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	algo, err := cryptosuites.AlgorithmForKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(config.AuthPreferenceGetter),
		cryptosuites.GitClient)
	if err != nil {
		return nil, trace.Wrap(err, "getting signing algorithm")
	}
	sshKey, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(algo)
	if err != nil {
		return nil, trace.Wrap(err, "generating SSH key")
	}
	resp, err := config.GitHubUserCertGenerator.GenerateGitHubUserCert(ctx, &integrationv1.GenerateGitHubUserCertRequest{
		Integration: config.Server.GetGitHub().Integration,
		PublicKey:   sshKey.MarshalSSHPublicKey(),
		UserId:      config.GitHubUserID,
		KeyId:       config.TeleportUser,
		Ttl:         durationpb.New(config.certTTL()),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(greedy52) cache it for TTL.
	signer, err := sshutils.NewSigner(sshKey.PrivateKeyPEM(), resp.AuthorizedKey)
	return signer, trace.Wrap(err)
}

const defaultGitHubUserCertTTL = 10 * time.Minute
