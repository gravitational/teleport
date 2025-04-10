package workloadattest

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest/sigstore"
)

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
}

// NewSigstoreAttestor creates a new SigstoreAttestor with the given configuration.
func NewSigstoreAttestor(cfg SigstoreAttestorConfig, log *slog.Logger) (*SigstoreAttestor, error) {
	keychain, err := sigstore.Keychain(cfg.CredentialsPath)
	if err != nil {
		return nil, trace.Wrap(err, "loading credentials")
	}

	cache, err := lru.New[string, *workloadidentityv1.WorkloadAttrsSigstore](64)
	if err != nil {
		return nil, trace.Wrap(err, "building LRU cache")
	}

	regHosts := make([]string, len(cfg.AdditionalRegistries))
	for idx, reg := range cfg.AdditionalRegistries {
		regHosts[idx] = reg.Host
	}

	return &SigstoreAttestor{
		cfg:           cfg,
		log:           log,
		registryHosts: regHosts,
		keychain:      keychain,
		cache:         cache,
	}, nil
}

// Attest discovers signatures and attestations for the given container.
func (a *SigstoreAttestor) Attest(ctx context.Context, ctr Container) (*workloadidentityv1.WorkloadAttrsSigstore, error) {
	logger := a.log.With(
		"image", ctr.GetImage(),
		"image_digest", ctr.GetImageDigest(),
	)
	logger.InfoContext(ctx, "Starting Sigstore workload attestation")

	if cached, ok := a.cache.Get(ctr.GetImageDigest()); ok {
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
			Logger:               a.log,
			Keychain:             a.keychain,
			AdditionalRegistries: a.registryHosts,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err, "discovering Sigstore payloads")
	}

	result := &workloadidentityv1.WorkloadAttrsSigstore{
		Payloads: payloads,
	}
	_ = a.cache.Add(ctr.GetImageDigest(), result)
	return result, nil
}

// EvictFromCache evicts the given container's signatures and attestations from
// the cache. It's called when getting a workload identity fails, in case the
// server's `SigstorePolicy`s have changed to require a newly-added signature.
func (a *SigstoreAttestor) EvictFromCache(ctx context.Context, ctr Container) {
	a.log.DebugContext(ctx,
		"Evicting image from Sigstore attestor cache",
		"image", ctr.GetImage(),
		"image_digest", ctr.GetImageDigest(),
	)
	a.cache.Remove(ctr.GetImageDigest())
}

// Container is satisfied by the WorkloadAttrsDockerContainer, WorkloadAttrsKubernetesContainer
// and WorkloadAttrsPodmanContainer protobuf messages.
type Container interface {
	// GetImage returns the name of the image (optionally including the registry).
	GetImage() string

	// GetImageDigest returns the exact digest/hash of the image.
	GetImageDigest() string
}
