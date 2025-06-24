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

package workloadidentity

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"go.opentelemetry.io/otel"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	trustv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/spiffe")

type BundleSet struct {
	// Local is the trust bundle for the local trust domain.
	Local *spiffebundle.Bundle
	// Federated is a map of trust domains to trust bundles.
	// It is keyed by the trust domain name
	// (the name of the SPIFFEFederation resource) and excludes the spiffe://
	// prefix.
	Federated map[string]*spiffebundle.Bundle
	stale     chan struct{}
}

// Stale returns a channel that will be closed when the BundleSet is stale
// and a new BundleSet is available.
func (b *BundleSet) Stale() <-chan struct{} {
	return b.stale
}

// Clone returns a deep copy of the BundleSet.
func (b *BundleSet) Clone() *BundleSet {
	clone := &BundleSet{
		Local:     b.Local.Clone(),
		Federated: make(map[string]*spiffebundle.Bundle),
		stale:     b.stale,
	}
	for k, v := range b.Federated {
		clone.Federated[k] = v.Clone()
	}
	return clone
}

// Equal returns true if the two BundleSets are equal.
func (b *BundleSet) Equal(other *BundleSet) bool {
	if len(b.Federated) != len(other.Federated) {
		return false
	}
	for k, v := range b.Federated {
		otherBundle, ok := other.Federated[k]
		if !ok {
			return false
		}
		if !v.Equal(otherBundle) {
			return false
		}
	}
	// go-spiffe's Equal method correctly handles nils of either value.
	return b.Local.Equal(other.Local)
}

// MarshalX509Bundle converts a trust bundle's certs to raw bytes.
// What's particularly special is that the certs are not pem encoded and
// are appended directly to one another. This is the way that the SPIFFE
// workload API clients expect.
func MarshalX509Bundle(b *x509bundle.Bundle) []byte {
	out := []byte{}
	for _, cert := range b.X509Authorities() {
		out = append(out, cert.Raw...)
	}
	return out
}

// EncodedX509Bundles returns a map of trust domain names to their trust bundles
// encoded as raw bytes. If includeLocal is true, the local trust domain will be
// included in the output. Uses MarshalX509Bundle to encode the bundles for
// compatibility with the SPIFFE workload API specification.
func (b *BundleSet) EncodedX509Bundles(includeLocal bool) map[string][]byte {
	bundles := make(map[string][]byte)
	if includeLocal {
		bundles[b.Local.TrustDomain().IDString()] = MarshalX509Bundle(b.Local.X509Bundle())
	}
	for _, v := range b.Federated {
		bundles[v.TrustDomain().IDString()] = MarshalX509Bundle(v.X509Bundle())
	}
	return bundles
}

// MarshaledJWKSBundles returns a map of trust domain names to their JWT-SVID
// signing keys encoded in the RFC 7517 JWKS format. If includeLocal is true,
// the local trust domain will be included in the output.
func (b *BundleSet) MarshaledJWKSBundles(includeLocal bool) (map[string][]byte, error) {
	bundles := make(map[string][]byte)
	if includeLocal {
		marshaled, err := b.Local.JWTBundle().Marshal()
		if err != nil {
			return nil, trace.Wrap(err, "marshaling local trust bundle")
		}
		bundles[b.Local.TrustDomain().IDString()] = marshaled
	}
	for _, v := range b.Federated {
		marshaled, err := v.JWTBundle().Marshal()
		if err != nil {
			return nil, trace.Wrap(
				err,
				"marshaling federated trust bundle (%s)",
				v.TrustDomain().Name(),
			)
		}
		bundles[v.TrustDomain().IDString()] = marshaled
	}
	return bundles, nil
}

// GetJWTBundleForTrustDomain returns the JWT bundle for the given trust domain.
// Implements the jwtbundle.Source interface.
func (b *BundleSet) GetJWTBundleForTrustDomain(trustDomain spiffeid.TrustDomain) (*jwtbundle.Bundle, error) {
	if trustDomain.Name() == b.Local.TrustDomain().Name() {
		return b.Local.JWTBundle(), nil
	}
	if bundle, ok := b.Federated[trustDomain.Name()]; ok {
		return bundle.JWTBundle(), nil
	}
	return nil, trace.NotFound("trust domain %q not found", trustDomain.Name())
}

type eventsWatcher interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// TrustBundleCache maintains a local, subscribable cache of trust domains and
// their trust bundles. It can be shared by multiple services within tbot and
// can leverage the main bot identity.
//
// This code should place a priority on continuance of service to subscribed
// workloads over strict correctness. If a confusing event is received, it is
// preferable to serve the last-good value than to disrupt subscribed workloads
// ability to communicate.
type TrustBundleCache struct {
	federationClient   machineidv1pb.SPIFFEFederationServiceClient
	trustClient        trustv1.TrustServiceClient
	eventsClient       eventsWatcher
	clusterName        string
	botIdentityReadyCh <-chan struct{}

	logger *slog.Logger

	mu        sync.RWMutex
	bundleSet *BundleSet
	// initialized will close when the cache is fully initialized.
	initialized chan struct{}
}

// String returns a string representation of the TrustBundleCache. Implements
// the tbot Service interface and fmt.Stringer interface.
func (m *TrustBundleCache) String() string {
	return "spiffe-trust-bundle-cache"
}

// TrustBundleCacheConfig is the configuration for a TrustBundleCache.
type TrustBundleCacheConfig struct {
	FederationClient   machineidv1pb.SPIFFEFederationServiceClient
	TrustClient        trustv1.TrustServiceClient
	EventsClient       eventsWatcher
	ClusterName        string
	Logger             *slog.Logger
	BotIdentityReadyCh <-chan struct{}
}

// NewTrustBundleCache creates a new TrustBundleCache.
func NewTrustBundleCache(cfg TrustBundleCacheConfig) (*TrustBundleCache, error) {
	switch {
	case cfg.FederationClient == nil:
		return nil, trace.BadParameter("missing FederationClient")
	case cfg.TrustClient == nil:
		return nil, trace.BadParameter("missing TrustClient")
	case cfg.EventsClient == nil:
		return nil, trace.BadParameter("missing EventsClient")
	case cfg.ClusterName == "":
		return nil, trace.BadParameter("missing ClusterName")
	case cfg.Logger == nil:
		return nil, trace.BadParameter("missing Logger")
	}
	return &TrustBundleCache{
		federationClient:   cfg.FederationClient,
		trustClient:        cfg.TrustClient,
		eventsClient:       cfg.EventsClient,
		clusterName:        cfg.ClusterName,
		logger:             cfg.Logger,
		botIdentityReadyCh: cfg.BotIdentityReadyCh,
		initialized:        make(chan struct{}),
	}, nil
}

const trustBundleInitFailureBackoff = 10 * time.Second

// Run initializes the cache and begins watching for events. It will block until
// the context is canceled, at which point it will return nil.
// Implements the tbot Service interface.
func (m *TrustBundleCache) Run(ctx context.Context) error {
	if m.botIdentityReadyCh != nil {
		select {
		case <-m.botIdentityReadyCh:
		default:
			m.logger.InfoContext(ctx, "Waiting for internal bot identity to be renewed before running")
			select {
			case <-m.botIdentityReadyCh:
			case <-ctx.Done():
				return nil
			}
		}
	}

	for {
		m.logger.InfoContext(
			ctx,
			"Initializing cache",
		)
		if err := m.watch(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			m.logger.ErrorContext(
				ctx,
				"Cache failed, will attempt to re-initialize after back off",
				"error", err,
				"backoff", trustBundleInitFailureBackoff,
			)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(trustBundleInitFailureBackoff):
			continue
		}
	}
}

func (m *TrustBundleCache) watch(ctx context.Context) error {
	watcher, err := m.eventsClient.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{
				// Only watch our local cert authority, we rely on the
				// SPIFFEFederation resource for all non-local clusters.
				Kind:        types.KindCertAuthority,
				LoadSecrets: false,
				Filter: types.CertAuthorityFilter{
					types.SPIFFECA: m.clusterName,
				}.IntoMap(),
			},
			{
				Kind:        types.KindSPIFFEFederation,
				LoadSecrets: false,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err, "establishing watcher")
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			m.logger.ErrorContext(
				ctx,
				"Failed to close watcher",
				"error", err,
			)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			return trace.BadParameter("unexpected event type: %v", event.Type)
		}
	case <-watcher.Done():
		return trace.Wrap(watcher.Error(), "watcher closed before initialization")
	}

	// Now that we know our watcher is streaming events, we can fetch the
	// current point-in-time list of resources.
	bundleSet, err := FetchInitialBundleSet(
		ctx,
		m.logger,
		m.federationClient,
		m.trustClient,
		true,
		m.clusterName,
	)
	if err != nil {
		return trace.Wrap(err, "fetching initial bundle set")
	}

	// The initial state of the bundleSet is now complete, we can set it.
	m.setBundleSet(bundleSet)

	m.logger.InfoContext(ctx, "Successfully initialized trust bundle cache")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt := <-watcher.Events():
			m.processEvent(ctx, evt)
		case <-watcher.Done():
			return trace.Wrap(watcher.Error(), "watcher closed")
		}
	}
}

func (m *TrustBundleCache) getBundleSet() *BundleSet {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.bundleSet == nil {
		return nil
	}
	// Clone so a receiver cannot mutate the current state without calling
	// setBundleSet.
	return m.bundleSet.Clone()
}

func (m *TrustBundleCache) setBundleSet(bundleSet *BundleSet) {
	m.mu.Lock()
	old := m.bundleSet

	// Clone the bundle set to avoid the caller mutating the state after it has
	// been set.
	m.bundleSet = bundleSet.Clone()
	m.bundleSet.stale = make(chan struct{})

	if old == nil {
		// Indicate that the first bundle set is now available.
		close(m.initialized)
	} else {
		// Indicate that a new bundle set is available.
		close(old.stale)
	}
	m.mu.Unlock()
}

// GetBundleSet returns the current BundleSet. If the cache is not yet
// initialized, it will block until it is.
func (m *TrustBundleCache) GetBundleSet(
	ctx context.Context,
) (*BundleSet, error) {
	select {
	case <-m.initialized:
		return m.getBundleSet(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *TrustBundleCache) processEvent(ctx context.Context, event types.Event) {
	// TODO(noah): Since we're only calling this from one goroutine, we could
	// probably use the previously modified value rather than rlocking and
	// cloning again.
	bundleSet := m.getBundleSet()

	log := m.logger.With("event_type", event.Type)
	if event.Resource != nil {
		log = log.With(
			"event_resource_kind", event.Resource.GetKind(),
			"event_resource_name", event.Resource.GetName(),
		)
	}

	switch event.Type {
	case types.OpDelete:
		switch event.Resource.GetKind() {
		case types.KindCertAuthority:
			// We don't expect this to ever happen under normal circumstances.
			// We'll simply ignore this since it would put consumers into a
			// weird state to not have a trust bundle for the local trust domain
			// available.
			log.WarnContext(
				ctx,
				"Ignoring event indicating CA deletion",
			)
		case types.KindSPIFFEFederation:
			_, ok := bundleSet.Federated[event.Resource.GetName()]
			if !ok {
				// Doesn't exist locally so nothing to change...
				return
			}
			log.InfoContext(
				ctx, "Processed deletion for federated trust bundle",
			)
			delete(bundleSet.Federated, event.Resource.GetName())
			m.setBundleSet(bundleSet)
		}
	case types.OpPut:
		switch event.Resource.GetKind() {
		case types.KindCertAuthority:
			ca, ok := event.Resource.(types.CertAuthority)
			if !ok {
				log.WarnContext(
					ctx,
					"Event did not contain expected resource type",
					"got", reflect.TypeOf(event.Resource),
				)
				return
			}
			if ca.GetType() != types.SPIFFECA {
				// Safeguard against receiving an event not for the SPIFFE CA.
				log.WarnContext(
					ctx,
					"Ignoring event for non-SPIFFE CA",
					"type", ca.GetType(),
				)
				return
			}
			if ca.GetClusterName() != m.clusterName {
				// Safeguard against receiving an event for a different cluster.
				log.WarnContext(
					ctx,
					"Ignoring event for different cluster",
					"cluster", ca.GetClusterName(),
				)
				return
			}
			log.DebugContext(
				ctx,
				"Processing update for local trust bundle",
				"trusted_tls_key_pairs", len(ca.GetTrustedTLSKeyPairs()),
			)

			bundle, err := convertSPIFFECAToBundle(ca)
			if err != nil {
				// This is "bad". Ideally, this situation should never occur,
				// but if it does, it's preferable that subscribed workloads
				// continue to use the last good bundle.
				log.WarnContext(
					ctx,
					"Failed to convert SPIFFE CA to trust bundle",
					"error", err,
				)
				return
			}

			if bundleSet.Local.Equal(bundle) {
				log.DebugContext(
					ctx,
					"Event resulted in no change to local trust bundle, ignoring",
				)
				return
			}
			log.InfoContext(
				ctx,
				"Processed update for local trust bundle",
				"x509_authorities", len(bundle.X509Authorities()),
			)
			bundleSet.Local = bundle
			m.setBundleSet(bundleSet)
		case types.KindSPIFFEFederation:
			r153, ok := event.Resource.(types.Resource153UnwrapperT[*machineidv1pb.SPIFFEFederation])
			if !ok {
				log.WarnContext(
					ctx,
					"Event did not contain a 153 style resource",
					"got", reflect.TypeOf(event.Resource),
				)
				return
			}
			federation := r153.UnwrapT()
			log.DebugContext(
				ctx,
				"Processing update for federated trust bundle",
			)

			bundle, err := convertSPIFFEFederationToBundle(federation)
			if err != nil {
				// TODO: Should we match the behavior for the local trust
				// bundle that's derived from the CA - i.e continue to use the
				// last good bundle, or, should we remove this from our local
				// set and tell workloads to start ignoring this trust domain?
				log.WarnContext(
					ctx,
					"Failed to convert SPIFFEFederation to trust bundle",
					"error", err,
				)
				return
			}

			if existingBundle, ok := bundleSet.Federated[federation.Metadata.Name]; ok && existingBundle.Equal(bundle) {
				log.DebugContext(
					ctx,
					"Event resulted in no change to federated trust bundle, ignoring",
				)
				return
			}
			log.InfoContext(
				ctx,
				"Processed update for federated trust bundle",
				"x509_authorities", len(bundle.X509Authorities()),
			)
			bundleSet.Federated[federation.Metadata.Name] = bundle
			m.setBundleSet(bundleSet)
		}
	default:
		log.WarnContext(ctx, "Ignoring unexpected event type")
	}
}

// FetchInitialBundleSet fetches a BundleSet of trust bundles from the Auth
// Server. If fetchFederatedBundles is true, then federated trust bundles will
// also be included as well as the trust bundle for the local trust domain.
func FetchInitialBundleSet(
	ctx context.Context,
	log *slog.Logger,
	federationClient machineidv1pb.SPIFFEFederationServiceClient,
	trustClient trustv1.TrustServiceClient,
	fetchFederatedBundles bool,
	clusterName string,
) (*BundleSet, error) {
	ctx, span := tracer.Start(
		ctx,
		"FetchInitialBundleSet",
	)
	defer span.End()

	bs := &BundleSet{
		Federated: make(map[string]*spiffebundle.Bundle),
	}
	spiffeCA, err := trustClient.GetCertAuthority(ctx, &trustv1.GetCertAuthorityRequest{
		Type:       string(types.SPIFFECA),
		Domain:     clusterName,
		IncludeKey: false,
	})
	if err != nil {
		return nil, trace.Wrap(err, "fetching spiffe CA")
	}
	bs.Local, err = convertSPIFFECAToBundle(spiffeCA)
	if err != nil {
		return nil, trace.Wrap(err, "converting SPIFFE CA to trust bundle")
	}

	if fetchFederatedBundles {
		spiffeFederations, err := listAllSPIFFEFederations(
			ctx, federationClient,
		)
		if err != nil {
			return nil, trace.Wrap(err, "fetching SPIFFE federations")
		}
		for _, federation := range spiffeFederations {
			bundle, err := convertSPIFFEFederationToBundle(federation)
			if err != nil {
				log.WarnContext(
					ctx,
					"Failed to convert SPIFFEFederation to trust bundle, it may not be ready yet",
					"trust_domain", federation.GetMetadata().Name,
					"error", err,
				)
				continue
			}
			bs.Federated[federation.Metadata.Name] = bundle
		}
	}

	return bs, nil
}

func listAllSPIFFEFederations(
	ctx context.Context,
	client machineidv1pb.SPIFFEFederationServiceClient,
) ([]*machineidv1pb.SPIFFEFederation, error) {
	var spiffeFeds []*machineidv1pb.SPIFFEFederation
	var token string
	for {
		res, err := client.ListSPIFFEFederations(ctx, &machineidv1pb.ListSPIFFEFederationsRequest{
			PageSize:  100,
			PageToken: token,
		})
		if err != nil {
			return nil, trace.Wrap(err, "listing SPIFFEFederations")
		}
		spiffeFeds = append(spiffeFeds, res.SpiffeFederations...)
		if res.NextPageToken == "" {
			break
		}
		token = res.NextPageToken
	}
	return spiffeFeds, nil
}

func convertSPIFFECAToBundle(ca types.CertAuthority) (*spiffebundle.Bundle, error) {
	td, err := spiffeid.TrustDomainFromString(ca.GetClusterName())
	if err != nil {
		return nil, trace.Wrap(err, "parsing trust domain name")
	}

	bundle := spiffebundle.New(td)

	// Add X509 authorities to the trust bundle.
	for _, certBytes := range services.GetTLSCerts(ca) {
		block, _ := pem.Decode(certBytes)
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err, "parsing cert")
		}
		bundle.AddX509Authority(cert)
	}

	// Add JWT authorities to the trust bundle.
	for _, keyPair := range ca.GetTrustedJWTKeyPairs() {
		pubKey, err := keys.ParsePublicKey(keyPair.PublicKey)
		if err != nil {
			return nil, trace.Wrap(err, "parsing public key")
		}
		kid, err := jwt.KeyID(pubKey)
		if err != nil {
			return nil, trace.Wrap(err, "generating key ID")
		}
		if err := bundle.AddJWTAuthority(kid, pubKey); err != nil {
			return nil, trace.Wrap(err, "adding JWT authority to bundle")
		}
	}

	return bundle, nil
}

func convertSPIFFEFederationToBundle(
	federation *machineidv1pb.SPIFFEFederation,
) (*spiffebundle.Bundle, error) {
	if federation.Status == nil {
		return nil, trace.BadParameter("federation missing status")
	}
	if federation.Status.CurrentBundle == "" {
		return nil, trace.BadParameter("federation missing status.current_bundle")
	}

	td, err := spiffeid.TrustDomainFromString(federation.Metadata.Name)
	if err != nil {
		return nil, trace.Wrap(err, "parsing trust domain name")
	}

	bundle, err := spiffebundle.Parse(td, []byte(federation.Status.CurrentBundle))
	if err != nil {
		return nil, trace.Wrap(err, "parsing bundle")
	}

	return bundle, nil
}
