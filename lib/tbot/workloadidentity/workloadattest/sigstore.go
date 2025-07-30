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

package workloadattest

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/sigstore"
)

// Maximum interval at which we'll attempt to discover signatures for a given
// image digest. This is used to avoid hammering registries if a workload is
// failing to get an identity in a crash-loop.
const sigstoreMaxRefreshInterval = 30 * time.Second

// SigstoreAttestorConfig holds the configuration for the Sigstore workload attestor.
type SigstoreAttestorConfig struct {
	// Enabled determines whether Sigstore workload attestation will be performed.
	Enabled bool `yaml:"enabled"`

	// AdditionalRegistries are OCI registries that will be checked for
	// signatures and attestations, in addition to the container image's source
	// registry.
	AdditionalRegistries []SigstoreRegistryConfig `yaml:"additional_registries,omitempty"`

	// CredentialsPath is a path to a Docker or Podman configuration file which
	// will be used to find registry credentials. If it's unset, we'll look in
	// the default locations (e.g. `$HOME/.docker/config.json`).
	CredentialsPath string `yaml:"credentials_path,omitempty"`

	// AllowedPrivateNetworkPrefixes configures the private network prefixes at
	// which registries can be reached. By default, the attestor will refuse to
	// connect to registries hosted at IPs designated for private use (e.g. 10.0.0.0/8)
	// in order to mitigate SSRF attacks.
	AllowedPrivateNetworkPrefixes []string `yaml:"allowed_private_network_prefixes,omitempty"`
}

func (s SigstoreAttestorConfig) CheckAndSetDefaults() error {
	if !s.Enabled {
		return nil
	}

	if s.CredentialsPath != "" {
		fi, err := os.Stat(s.CredentialsPath)
		if err != nil {
			return trace.Wrap(err, "checking credentials_path")
		}
		if fi.IsDir() {
			return trace.BadParameter("credentials_path cannot be a directory")
		}
	}

	for idx, reg := range s.AdditionalRegistries {
		field := fmt.Sprintf("additional_registries[%d].host", idx)

		if reg.Host == "" {
			return trace.BadParameter("%s cannot be blank", field)
		}

		if _, err := name.NewRegistry(reg.Host); err != nil {
			return trace.Wrap(err, "parsing %s", field)
		}
	}

	for idx, prefix := range s.AllowedPrivateNetworkPrefixes {
		if _, err := netip.ParsePrefix(prefix); err != nil {
			return trace.Wrap(err, "parsing allowed_private_network_prefixes[%d])", idx)
		}
	}

	return nil
}

// SigstoreRegistryConfig holds configuration for an OCI registry the Sigstore
// attestor will check for signatures and attestations.
type SigstoreRegistryConfig struct {
	// Host is the hostname (and optionally the port) of the registry.
	Host string `yaml:"host"`
}

// SigstoreAttestor discovers signatures and attestations for a container image.
type SigstoreAttestor struct {
	cfg SigstoreAttestorConfig
	log *slog.Logger

	registryHosts []string

	keychain authn.Keychain
	cache    *lru.Cache[string, *workloadidentityv1.WorkloadAttrsSigstore]

	maxRefreshInterval time.Duration
	failuresMu         sync.Mutex
	failures           map[string]time.Time
}

// NewSigstoreAttestor creates a new SigstoreAttestor with the given configuration.
func NewSigstoreAttestor(cfg SigstoreAttestorConfig, log *slog.Logger) (*SigstoreAttestor, error) {
	keychain, err := sigstore.Keychain(cfg.CredentialsPath)
	if err != nil {
		return nil, trace.Wrap(err, "loading credentials")
	}

	regHosts := make([]string, len(cfg.AdditionalRegistries))
	for idx, reg := range cfg.AdditionalRegistries {
		regHosts[idx] = reg.Host
	}

	att := &SigstoreAttestor{
		cfg:                cfg,
		log:                log,
		registryHosts:      regHosts,
		keychain:           keychain,
		maxRefreshInterval: sigstoreMaxRefreshInterval,
		failures:           make(map[string]time.Time),
	}
	att.cache, err = lru.NewWithEvict[string, *workloadidentityv1.WorkloadAttrsSigstore](64, att.onCacheEviction)
	if err != nil {
		return nil, trace.Wrap(err, "building LRU cache")
	}
	return att, nil

}

// Attest discovers signatures and attestations for the given container.
func (a *SigstoreAttestor) Attest(ctx context.Context, ctr Container) (*workloadidentityv1.WorkloadAttrsSigstore, error) {
	logger := a.log.With(
		"image", ctr.GetImage(),
		"image_digest", ctr.GetImageDigest(),
	)
	logger.InfoContext(ctx, "Starting Sigstore workload attestation")

	if cached := a.getCached(ctr.GetImageDigest()); cached != nil {
		logger.DebugContext(ctx, "Sigstore attestor cache hit")
		return cached, nil
	}

	logger.DebugContext(ctx, "Sigstore attestor cache miss")

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	payloads, err := sigstore.Discover(ctx,
		ctr.GetImage(),
		ctr.GetImageDigest(),
		sigstore.DiscoveryConfig{
			Logger:                        a.log,
			Keychain:                      a.keychain,
			AdditionalRegistries:          a.registryHosts,
			AllowedPrivateNetworkPrefixes: a.cfg.AllowedPrivateNetworkPrefixes,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "discovering Sigstore payloads")
	}

	result := &workloadidentityv1.WorkloadAttrsSigstore{
		Payloads: payloads,
	}
	_ = a.cache.Add(ctr.GetImageDigest(), result)

	a.failuresMu.Lock()
	delete(a.failures, ctr.GetImageDigest())
	a.failuresMu.Unlock()

	return result, nil
}

// MarkFailed tracks that we failed to get a workload identity for the given
// container. After sigstoreMaxRefreshInterval has elapsed, we will attempt to
// refresh the signatures and bundles.
func (a *SigstoreAttestor) MarkFailed(ctx context.Context, ctr Container) {
	a.log.DebugContext(ctx,
		"Marking image digest as failed in Sigstore attestor cache",
		"image", ctr.GetImage(),
		"image_digest", ctr.GetImageDigest(),
	)
	a.failuresMu.Lock()
	defer a.failuresMu.Unlock()

	if _, ok := a.failures[ctr.GetImageDigest()]; !ok {
		a.failures[ctr.GetImageDigest()] = time.Now()
	}
}

func (a *SigstoreAttestor) getCached(imageDigest string) *workloadidentityv1.WorkloadAttrsSigstore {
	cached, ok := a.cache.Get(imageDigest)
	if !ok {
		return nil
	}

	a.failuresMu.Lock()
	defer a.failuresMu.Unlock()

	failedAt, failed := a.failures[imageDigest]
	if !failed || time.Since(failedAt) < a.maxRefreshInterval {
		return cached
	}
	return nil
}

func (a *SigstoreAttestor) onCacheEviction(imageDigest string, attrs *workloadidentityv1.WorkloadAttrsSigstore) {
	if a.cache.Contains(imageDigest) {
		return
	}

	a.failuresMu.Lock()
	delete(a.failures, imageDigest)
	a.failuresMu.Unlock()
}

// Container is satisfied by the WorkloadAttrsDockerContainer, WorkloadAttrsKubernetesContainer
// and WorkloadAttrsPodmanContainer protobuf messages.
type Container interface {
	// GetImage returns the name of the image (optionally including the registry).
	GetImage() string

	// GetImageDigest returns the exact digest/hash of the image.
	GetImageDigest() string
}
