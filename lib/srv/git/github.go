/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils"
)

// githubKeyDownloader downloads SSH keys from the GitHub meta API. The keys
// are used to verify GitHub server when forwarding Git commands to it.
type githubKeyDownloader struct {
	keys atomic.Pointer[[]ssh.PublicKey]

	logger      *slog.Logger
	apiEndpoint string
	clock       clockwork.Clock
}

// newGitHubKeyDownloader creates a new githubKeyDownloader.
func newGitHubKeyDownloader() *githubKeyDownloader {
	return &githubKeyDownloader{
		apiEndpoint: "https://api.github.com/meta",
		logger:      slog.With(teleport.ComponentKey, teleport.ComponentGit),
		clock:       clockwork.NewRealClock(),
	}
}

// Start starts a task that periodically downloads SSH keys from the GitHub meta
// API. The task is stopped when provided context is closed.
func (d *githubKeyDownloader) Start(ctx context.Context) {
	d.logger.InfoContext(ctx, "Starting GitHub key downloader")
	defer d.logger.InfoContext(ctx, "GitHub key downloader stopped")

	// Fire a refresh immediately then once a day afterward.
	timer := d.clock.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-timer.Chan():
			d.refreshWithRetries(ctx)
			timer.Reset(time.Hour * 24)
		case <-ctx.Done():
			return
		}
	}
}

// GetKnownKeys returns known server keys.
func (d *githubKeyDownloader) GetKnownKeys() ([]ssh.PublicKey, error) {
	keys := d.keys.Load()
	if keys == nil {
		return nil, trace.NotFound("server keys not found for github.com")
	}
	return *keys, nil
}

func (d *githubKeyDownloader) refreshWithRetries(ctx context.Context) {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(time.Second),
		Max:    time.Minute * 10,
		Jitter: retryutils.HalfJitter,
		Clock:  d.clock,
	})
	if err != nil {
		d.logger.WarnContext(ctx, "Failed to create retry", "error", err)
		return
	}

	for {
		if err := d.refresh(ctx); err != nil {
			d.logger.WarnContext(ctx, "Failed to download GitHub server keys", "error", err)
		} else {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-retry.After():
			retry.Inc()
		}
	}
}

func (d *githubKeyDownloader) refresh(ctx context.Context) error {
	d.logger.DebugContext(ctx, "Calling GitHub meta API", "endpoint", d.apiEndpoint)
	// Meta API reference:
	// https://docs.github.com/en/rest/meta/meta#get-github-meta-information
	req, err := http.NewRequestWithContext(ctx, "GET", d.apiEndpoint, nil)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := defaults.HTTPClient()
	if err != nil {
		return trace.Wrap(err, "creating HTTP client")
	}
	resp, err := client.Do(req)
	if err != nil {
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return trace.Wrap(err, "reading GitHub meta API response body")
	}

	meta := struct {
		SSHKeys []string `json:"ssh_keys"`
	}{}
	if err := json.Unmarshal(body, &meta); err != nil {
		return trace.Wrap(err, "decoding GitHub meta API response")
	}

	var keys []ssh.PublicKey
	for _, key := range meta.SSHKeys {
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
		if err != nil {
			return trace.Wrap(err, "parsing SSH public key")
		}
		keys = append(keys, publicKey)
	}

	d.keys.Store(&keys)
	d.logger.DebugContext(ctx, "Fetched GitHub metadata", "ssh_keys", meta.SSHKeys)
	return nil
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
	userTTL := c.IdentityExpires.Sub(c.Clock.Now())
	return min(userTTL, defaultGitHubUserCertTTL)
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

	signer, err := sshutils.NewSigner(sshKey.PrivateKeyPEM(), resp.AuthorizedKey)
	return signer, trace.Wrap(err)
}

const defaultGitHubUserCertTTL = 10 * time.Minute
