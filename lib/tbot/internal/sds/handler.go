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

package sds

import (
	"context"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"maps"
	"slices"
	"time"

	corev3pb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3pb "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discoveryv3pb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secretv3pb "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	workloadpb "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/utils"
)

// The following constants allow querying this API for SVIDs and trust bundles
// without knowing their names. These special values are relied upon by tools
// like Istio.
const (
	// EnvoyDefaultSVIDName indicates that the first available SVID should be
	// returned.
	EnvoyDefaultSVIDName = "default"
	// EnvoyDefaultBundleName indicates that the default trust bundle should be
	// returned, e.g the one for the trust domain that the workload is part of.
	EnvoyDefaultBundleName = "ROOTCA"
	// EnvoyAllBundlesName indicates that all available trust bundles,
	// including federated ones, should be returned.
	EnvoyAllBundlesName = "ALL"
)

// BundleSetGetter is an interface for retrieving trust bundle sets.
type BundleSetGetter interface {
	GetBundleSet(ctx context.Context) (*workloadidentity.BundleSet, error)
}

// SVIDFetcher is a function type for fetching X509 SVIDs.
type SVIDFetcher func(ctx context.Context, localBundle *spiffebundle.Bundle) ([]*workloadpb.X509SVID, error)

// ClientAuthenticator is a function type that authenticates clients and returns
// a logger and SVID fetcher for the authenticated client.
type ClientAuthenticator func(ctx context.Context) (*slog.Logger, SVIDFetcher, error)

// HandlerConfig contains the configuration for creating a new SDS handler.
type HandlerConfig struct {
	Logger              *slog.Logger
	RenewalInterval     time.Duration
	TrustBundleCache    BundleSetGetter
	ClientAuthenticator ClientAuthenticator
}

// CheckAndSetDefaults validates the HandlerConfig and sets default values.
func (cfg *HandlerConfig) CheckAndSetDefaults() error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.RenewalInterval <= 0 {
		return trace.BadParameter("RenewalInterval must be positive")
	}
	if cfg.TrustBundleCache == nil {
		return trace.BadParameter("TrustBundleCache is required")
	}
	if cfg.ClientAuthenticator == nil {
		return trace.BadParameter("ClientAuthenticator is required")
	}
	return nil
}

// Handler implements an Envoy SDS API.
//
// This effectively replaces the Workload API for Envoy, but functions in a
// very similar way.
type Handler struct {
	log                 *slog.Logger
	renewalInterval     time.Duration
	trustBundleCache    BundleSetGetter
	clientAuthenticator ClientAuthenticator
}

// NewHandler creates a new SDS handler with the provided configuration.
func NewHandler(cfg HandlerConfig) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "validating handler config")
	}

	return &Handler{
		log:                 cfg.Logger,
		renewalInterval:     cfg.RenewalInterval,
		trustBundleCache:    cfg.TrustBundleCache,
		clientAuthenticator: cfg.ClientAuthenticator,
	}, nil
}

// FetchSecrets implements
// envoy.service.secret.v3/SecretDiscoveryService.FetchSecrets.
// This should return the current SVIDs and trust bundles.
//
// See:
// - DiscoveryRequest - https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#service-discovery-v3-discoveryrequest
// - DiscoveryResponse - https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#service-discovery-v3-discoveryresponse
func (h *Handler) FetchSecrets(
	ctx context.Context,
	req *discoveryv3pb.DiscoveryRequest,
) (*discoveryv3pb.DiscoveryResponse, error) {
	if err := enforceMinimumEnvoyVersion(req); err != nil {
		return nil, trace.Wrap(err)
	}

	log, fetchSVIDs, err := h.clientAuthenticator(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "authenticating client")
	}

	log.DebugContext(
		ctx,
		"SecretDiscoveryService.FetchSecrets request received from workload",
		slog.Group("req", "resource_names", req.ResourceNames),
	)
	defer log.DebugContext(ctx, "SecretDiscoveryService.FetchSecrets request handled")

	bundleSet, err := h.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting trust bundle set")
	}

	svids, err := fetchSVIDs(ctx, bundleSet.Local)
	if err != nil {
		return nil, trace.Wrap(err, "fetching X509 SVIDs")
	}

	return h.generateResponse(
		bundleSet,
		svids,
		req,
	)
}

// StreamSecrets implements
// envoy.service.secret.v3/SecretDiscoveryService.StreamSecrets.
// This should return the current SVIDs and CAs, and stream any future updates.
//
// This is a little more complex than one might expect since Envoy provides
// an ACK/NACK mechanism to indicate that the config included within a response
// was successfully applied.
//
// From the docs on the request version_info field:
//
//	The version_info provided in the request messages will be the
//	version_info received with the most recent successfully processed
//	response or empty on the first request. It is expected that no new
//	request is sent after a response is received until the Envoy
//	instance is ready to ACK/NACK the new configuration. ACK/NACK takes
//	place by returning the new API config version as applied or the
//	previous API config version respectively.
//
// From the docs on the DiscoveryRequest.response_nonce field:
//
//	nonce corresponding to DiscoveryResponse being ACK/NACKed. See above
//	discussion on version_info and the DiscoveryResponse nonce comment
//
// From the docs on the DiscoveryResponse.nonce field:
//
//	For gRPC based subscriptions, the nonce provides a way to explicitly ack a
//	specific DiscoveryResponse in a following DiscoveryRequest. Additional
//	messages may have been sent by Envoy to the management server for the
//	previous version on the stream prior to this DiscoveryResponse, that were
//	unprocessed at response send time. The nonce allows the management server to
//	ignore any further DiscoveryRequests for the previous version until a
//	DiscoveryRequest bearing the nonce.
//
// See also:
// - DiscoveryRequest - https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#service-discovery-v3-discoveryrequest
// - DiscoveryResponse - https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#service-discovery-v3-discoveryresponse
//
// TODO: We could probably be smarter about how we handle the version e.g,
// do we need to send another response if we're just going to send identical
// certificates ??
func (h *Handler) StreamSecrets(
	srv secretv3pb.SecretDiscoveryService_StreamSecretsServer,
) error {
	ctx := srv.Context()
	log, fetchSVIDs, err := h.clientAuthenticator(ctx)
	if err != nil {
		return trace.Wrap(err, "authenticating client")
	}

	log.DebugContext(
		ctx,
		"SecretDiscoveryService.StreamSecrets stream started",
	)
	defer log.DebugContext(ctx, "SecretDiscoveryService.FetchSecrets stream finished")

	bundleSet, err := h.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return trace.Wrap(err, "getting trust bundle set")
	}

	// Push incoming messages into a chan for the main loop to handle
	recvCh := make(chan *discoveryv3pb.DiscoveryRequest, 1)
	recvErrCh := make(chan error, 1)
	go func() {
		for {
			req, err := srv.Recv()
			if err != nil {
				// It's worth noting that we'll receive an IOF/Canceled error
				// here once the main goroutine has exited or the client goes
				// away.
				select {
				case recvErrCh <- err:
				default:
				}
				return
			}
			recvCh <- req
		}
	}()

	renewalTimer := time.NewTimer(h.renewalInterval)
	// Stop the timer immediately so we can start timing after the first
	// response is sent.
	renewalTimer.Stop()
	defer renewalTimer.Stop()

	// Track the last response and last request to allow us to handle ACK/NACK
	// and versioning.
	var (
		lastResp *discoveryv3pb.DiscoveryResponse
		lastReq  *discoveryv3pb.DiscoveryRequest
		svids    []*workloadpb.X509SVID
	)
	for {
		select {
		case err := <-recvErrCh:
			// If we receive an error from the read side, we should exit.
			// This error could be an EOF/Canceled error if the client
			// goes away.
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return trace.Wrap(err)
		case newReq := <-recvCh:
			log.DebugContext(
				ctx,
				"Received StreamSecrets DiscoveryRequest",
				slog.Group(
					"req",
					"resource_names", newReq.ResourceNames,
					"version_info", newReq.VersionInfo,
					"response_nonce", newReq.ResponseNonce,
					"node_id", newReq.Node.GetId(),
				),
			)

			shouldRespond := true

			// If we've sent a response, then this request ought to be a reply
			// We should check the nonces/versions to ensure it's successfully
			// applied
			if lastResp != nil {
				// Envoy can send an "ErrorDetails" which indicates that the
				// previously sent response could not be applied.
				if newReq.ErrorDetail != nil {
					log.WarnContext(
						ctx,
						"Envoy was unable to apply previous discovery response",
						"error", newReq.ErrorDetail.Message,
					)
				}

				if lastResp.Nonce != newReq.ResponseNonce {
					log.WarnContext(
						ctx,
						"Envoy sent a nonce which does not match the last response, ignoring request",
						"want", lastResp.Nonce,
						"got", newReq.ResponseNonce,
					)
					// We want to ignore this request because it's been sent
					// before Envoy has processed our last response.
					continue
				}

				// Since we've already sent them a response, we should only
				// respond again if they've changed what they've requested.
				shouldRespond = !elementsMatch(
					lastReq.ResourceNames, newReq.ResourceNames,
				)
				// TODO(noah): SPIRE's implementation seems to check the last
				// requests resource names - but I wonder if it makes more sense
				// to compare the requests resource names against those sent in
				// the last response.
			}

			lastReq = newReq
			if !shouldRespond {
				continue
			}

		case <-bundleSet.Stale():
			newBundleSet, err := h.trustBundleCache.GetBundleSet(ctx)
			if err != nil {
				return trace.Wrap(err, "getting trust bundle set")
			}
			if !newBundleSet.Local.Equal(bundleSet.Local) {
				// If the "local" trust domain's CA has changed, we need to
				// reissue the SVIDs.
				svids = nil
			}
			bundleSet = newBundleSet
		case <-renewalTimer.C:
			// Handle renewal time!
			log.DebugContext(ctx, "Renewing SVIDs for StreamSecrets stream")
		}

		// Fetch the SVIDs if necessary
		if svids == nil {
			svids, err = fetchSVIDs(ctx, bundleSet.Local)
			if err != nil {
				return trace.Wrap(err, "fetching X509 SVIDs")
			}
		}

		resp, err := h.generateResponse(
			bundleSet, svids, lastReq,
		)
		if err != nil {
			return trace.Wrap(err, "generating response")
		}

		// Decorate the generated response with the stream ACK/NACK and
		// versioning fields.
		nonce, err := utils.CryptoRandomHex(4)
		if err != nil {
			return trace.Wrap(err, "generating nonce")
		}
		resp.Nonce = nonce
		resp.VersionInfo = time.Now().UTC().Format(time.RFC3339Nano)
		if err := srv.Send(resp); err != nil {
			return trace.Wrap(err, "sending response")
		}
		lastResp = resp
		log.DebugContext(
			ctx,
			"Sent StreamSecrets DiscoveryResponse",
			slog.Group(
				"resp",
				"resources_len", len(resp.Resources),
				"nonce", resp.Nonce,
				"version_info", resp.VersionInfo,
			),
		)

		renewalTimer.Reset(h.renewalInterval)
	}
}

// DeltaSecrets implements
// envoy.service.secret.v3/SecretDiscoveryService.DeltaSecrets.
// We do not implement this method.
func (h *Handler) DeltaSecrets(
	_ secretv3pb.SecretDiscoveryService_DeltaSecretsServer,
) error {
	return status.Error(
		codes.Unimplemented,
		"method not implemented",
	)
}

// generateResponse generates a DiscoveryResponse for the given bundle set, SVIDs, and request.
func (h *Handler) generateResponse(
	bundleSet *workloadidentity.BundleSet,
	svids []*workloadpb.X509SVID,
	req *discoveryv3pb.DiscoveryRequest,
) (*discoveryv3pb.DiscoveryResponse, error) {
	// names holds all names requested by the client
	// if this is nothing, we assume they want everything available to them.
	// it's worth keeping in mind that there's some special names we need to
	// handle.
	names := map[string]bool{}
	for _, name := range req.ResourceNames {
		// Ignore empty string
		if name != "" {
			names[name] = true
		}
	}
	returnAll := len(names) == 0

	var resources []*anypb.Any
	for i, svid := range svids {
		// Now we need to filter the SVIDs down to those requested by the
		// client.
		// There's a special case here, if they've requested the default SVID,
		// we want to ensure that the first SVID is returned and its name
		// overrridden.

		switch {
		case returnAll || names[svid.SpiffeId]:
			secret, err := newTLSV3Certificate(svid, "")
			if err != nil {
				return nil, trace.Wrap(err, "creating TLS certificate")
			}
			resources = append(resources, secret)
			delete(names, svid.SpiffeId)
		case names[EnvoyDefaultSVIDName] && i == 0:
			secret, err := newTLSV3Certificate(svid, EnvoyDefaultSVIDName)
			if err != nil {
				return nil, trace.Wrap(err, "creating TLS certificate")
			}
			resources = append(resources, secret)
			delete(names, EnvoyDefaultSVIDName)
		}
	}

	// Convert trust bundle to SDS validator type.
	switch {
	case returnAll || names[bundleSet.Local.TrustDomain().IDString()]:
		// If this name was explicitly specified or no names were specified,
		// we use the proper trust domain ID as the resource name.
		validator, err := newTLSV3ValidationContext(
			[]*spiffebundle.Bundle{
				bundleSet.Local,
			}, bundleSet.Local.TrustDomain().IDString(),
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS validation context")
		}
		resources = append(resources, validator)
		delete(names, bundleSet.Local.TrustDomain().IDString())
	case names[EnvoyDefaultBundleName]:
		// If they've requested the default bundle, we send the connected trust
		// domain's bundle - but - we override the name to match what they
		// expect.
		validator, err := newTLSV3ValidationContext(
			[]*spiffebundle.Bundle{
				bundleSet.Local,
			}, EnvoyDefaultBundleName,
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS validation context")
		}
		resources = append(resources, validator)
		delete(names, EnvoyDefaultBundleName)
	case names[EnvoyAllBundlesName]:
		// Return all the trust bundles as part of a single validation context.
		// We'll also override the name to match what they requested.
		bundles := slices.Collect(maps.Values(bundleSet.Federated))
		bundles = append(bundles, bundleSet.Local)
		validator, err := newTLSV3ValidationContext(
			bundles, EnvoyAllBundlesName,
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS validation context")
		}
		resources = append(resources, validator)
		delete(names, EnvoyAllBundlesName)
	}

	if returnAll {
		for _, bundle := range bundleSet.Federated {
			validator, err := newTLSV3ValidationContext(
				[]*spiffebundle.Bundle{
					bundle,
				}, bundle.TrustDomain().IDString(),
			)
			if err != nil {
				return nil, trace.Wrap(err, "creating TLS validation context")
			}
			resources = append(resources, validator)
		}
	} else {
		// For any remaining names, see if they match any federated trust bundles.
		for name := range maps.Keys(names) {
			var found *spiffebundle.Bundle
			for _, bundle := range bundleSet.Federated {
				if name == bundle.TrustDomain().IDString() {
					found = bundle
					break
				}
			}
			if found != nil {
				validator, err := newTLSV3ValidationContext(
					[]*spiffebundle.Bundle{
						found,
					}, found.TrustDomain().IDString(),
				)
				if err != nil {
					return nil, trace.Wrap(err, "creating TLS validation context")
				}
				resources = append(resources, validator)
				delete(names, name)
			}
		}
	}

	// If any names are left-over, we've not been able to service them so
	// we should return an explicit error rather than omitting data.
	if len(names) > 0 {
		return nil, trace.BadParameter("unknown resource names: %v", slices.Collect(maps.Keys(names)))
	}

	return &discoveryv3pb.DiscoveryResponse{
		// Copy in requested type from req
		TypeUrl:   req.TypeUrl,
		Resources: resources,
	}, nil
}

// elementsMatch compares two string slices for equality, ignoring order.
func elementsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		if !seen[v] {
			return false
		}
	}
	return true
}

// enforceMinimumEnvoyVersion ensures that the client is running a minimum version of Envoy.
func enforceMinimumEnvoyVersion(req *discoveryv3pb.DiscoveryRequest) error {
	// Envoy 1.18 introduced the SPIFFE specific tls context validator - so
	// that's our minimum supported version.
	buildVersion := req.Node.GetUserAgentBuildVersion()
	// If not specified, let's assume that it's recent enough.
	if buildVersion == nil {
		return nil
	}
	if buildVersion.Version.MajorNumber > 1 {
		return nil
	}
	if buildVersion.Version.MinorNumber >= 18 {
		return nil
	}
	return trace.BadParameter("minimum supported version of envoy supported by the tbot SDS API is 1.18")
}

// newTLSV3Certificate creates a new TLS certificate secret for the given SVID.
func newTLSV3Certificate(
	svid *workloadpb.X509SVID, overrideResourceName string,
) (*anypb.Any, error) {
	// noah: This section of code does not currently support intermediate
	// certificates, but, we don't currently use them.
	certBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: svid.X509Svid,
	})
	privateKeyBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: svid.X509SvidKey,
	})

	// https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/common.proto#envoy-v3-api-msg-extensions-transport-sockets-tls-v3-tlscertificate
	secret := &tlsv3pb.Secret{
		Name: svid.SpiffeId,
		Type: &tlsv3pb.Secret_TlsCertificate{
			TlsCertificate: &tlsv3pb.TlsCertificate{
				CertificateChain: &corev3pb.DataSource{
					Specifier: &corev3pb.DataSource_InlineBytes{
						// Must be appended PEM-wrapped X509 certificates
						InlineBytes: certBytes,
					},
				},
				PrivateKey: &corev3pb.DataSource{
					Specifier: &corev3pb.DataSource_InlineBytes{
						// Must be PKCS8 PEM-wrapped private key
						InlineBytes: privateKeyBytes,
					},
				},
			},
		},
	}
	if overrideResourceName != "" {
		secret.Name = overrideResourceName
	}

	return anypb.New(secret)
}

const envoySPIFFECertValidator = "envoy.tls.cert_validator.spiffe"

// newTLSV3ValidationContext creates a new TLS validation context for the given bundles.
func newTLSV3ValidationContext(
	bundles []*spiffebundle.Bundle, resourceName string,
) (*anypb.Any, error) {
	var trustDomains []*tlsv3pb.SPIFFECertValidatorConfig_TrustDomain
	for _, bundle := range bundles {
		caBytes, err := bundle.X509Bundle().Marshal()
		if err != nil {
			return nil, trace.Wrap(err, "marshaling trust bundle")
		}

		// https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/tls_spiffe_validator_config.proto#extensions-transport-sockets-tls-v3-spiffecertvalidatorconfig-trustdomain
		trustDomain := &tlsv3pb.SPIFFECertValidatorConfig_TrustDomain{
			// From API reference:
			//   Note that this must not have "spiffe://" prefix.
			Name: bundle.TrustDomain().Name(),
			TrustBundle: &corev3pb.DataSource{
				Specifier: &corev3pb.DataSource_InlineBytes{
					// Must be concatenated PEM-wrapped X509 certificates
					InlineBytes: caBytes,
				},
			},
		}
		trustDomains = append(trustDomains, trustDomain)
	}

	// Generate the typed config for the SPIFFE TLS cert validator extension
	// https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/tls_spiffe_validator_config.proto
	extConfig, err := anypb.New(&tlsv3pb.SPIFFECertValidatorConfig{
		TrustDomains: trustDomains,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/common.proto#extensions-transport-sockets-tls-v3-certificatevalidationcontext
	secret := &tlsv3pb.Secret{
		// We intentionally use the full IDString here in contrast to the
		// short name used in the TrustDomain config block.
		Name: resourceName,
		Type: &tlsv3pb.Secret_ValidationContext{
			ValidationContext: &tlsv3pb.CertificateValidationContext{
				// We use a custom validation context for SPIFFE to provide
				// greater usability than a plain TLS CA.
				CustomValidatorConfig: &corev3pb.TypedExtensionConfig{
					Name:        envoySPIFFECertValidator,
					TypedConfig: extConfig,
				},
			},
		},
	}

	return anypb.New(secret)
}
