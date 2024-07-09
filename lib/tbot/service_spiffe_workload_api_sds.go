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

package tbot

import (
	"context"
	"encoding/pem"

	corev3pb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	tlsv3pb "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discoveryv3pb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secretv3pb "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	anyv1pb "github.com/golang/protobuf/ptypes/any"
	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	workloadpb "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

// Various code related to implementation of Envoy SDS API
//
// This effectively replaces the Workload API for Envoy, but functions in a
// very similar way.

// The following constants allow querying this API for SVIDs and trust bundles
// without knowing their names. These special values are relied upon by tools
// like Istio.
const (
	// envoyDefaultSVIDName indicates that the first available SVID should be
	// returned.
	envoyDefaultSVIDName = "default"
	// envoyDefaultBundleName indicates that the default trust bundle should be
	// returned, e.g the one for the trust domain that the workload is part of.
	envoyDefaultBundleName = "ROOTCA"
	// envoyAllBundlesName indicates that all available trust bundles,
	// including federated ones, should be returned.
	envoyAllBundlesName = "ALL"
)

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

func newTLSV3Certificate(
	svid *workloadpb.X509SVID, overrideResourceName string,
) (*anyv1pb.Any, error) {
	// TODO: Support intermediate certificates
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
		// Must be SPIFFE ID
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

func newTLSV3ValidationContext(
	tb *x509bundle.Bundle, overrideResourceName string,
) (*anyv1pb.Any, error) {
	caBytes, err := tb.Marshal()
	if err != nil {
		return nil, trace.Wrap(err, "marshaling trust bundle")
	}

	// https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/tls_spiffe_validator_config.proto#extensions-transport-sockets-tls-v3-spiffecertvalidatorconfig-trustdomain
	trustDomains := []*tlsv3pb.SPIFFECertValidatorConfig_TrustDomain{
		{
			// From API reference:
			//   Note that this must not have “spiffe://” prefix.
			Name: tb.TrustDomain().Name(),
			TrustBundle: &corev3pb.DataSource{
				Specifier: &corev3pb.DataSource_InlineBytes{
					// Must be concatenated PEM-wrapped X509 certificates
					InlineBytes: caBytes,
				},
			},
		},
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
		Name: tb.TrustDomain().IDString(),
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
	if overrideResourceName != "" {
		secret.Name = overrideResourceName
	}

	return anypb.New(secret)
}

// FetchSecrets implements
// envoy.service.secret.v3/SecretDiscoveryService.FetchSecrets.
// This should return the current SVIDs and trust bundles.
func (s *SPIFFEWorkloadAPIService) FetchSecrets(
	ctx context.Context,
	req *discoveryv3pb.DiscoveryRequest,
) (*discoveryv3pb.DiscoveryResponse, error) {
	if err := enforceMinimumEnvoyVersion(req); err != nil {
		return nil, trace.Wrap(err)
	}

	log, creds, err := s.authenticateClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "authenticating client")
	}

	log.InfoContext(ctx, "SecretDiscoveryService.FetchSecrets request received from workload")
	defer log.InfoContext(ctx, "SecretDiscoveryService.FetchSecrets request handled")

	tb := s.getTrustBundle()

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

	var resources []*anyv1pb.Any

	// Filter SVIDs down to those accessible to this workload
	availableSVIDs := filterSVIDRequests(ctx, log, s.cfg.SVIDs, creds)
	// Fetch the SVIDs and convert them into the SDS cert type.
	svids, err := s.fetchX509SVIDs(ctx, log, availableSVIDs)
	if err != nil {
		return nil, trace.Wrap(err, "fetching X509 SVIDs")
	}
	for i, svid := range svids {
		// Now we need to filter the SVIDs down to those requested by the
		// client.
		// There's a special case here, if they've requested the default SVID,
		// we want to ensure that the first SVID is returned and it's name
		// overrridden.

		switch {
		case returnAll || names[svid.SpiffeId]:
			secret, err := newTLSV3Certificate(svid, "")
			if err != nil {
				return nil, trace.Wrap(err, "creating TLS certificate")
			}
			resources = append(resources, secret)
		case names[envoyDefaultSVIDName] && i == 0:
			secret, err := newTLSV3Certificate(svid, envoyDefaultSVIDName)
			if err != nil {
				return nil, trace.Wrap(err, "creating TLS certificate")
			}
			resources = append(resources, secret)
		}
	}

	// Convert trust bundle to SDS validator type.
	switch {
	case returnAll || names[tb.TrustDomain().IDString()]:
		// If this name was explicitly specified or no names were specified,
		// we use the proper trust domain ID as the resource name.
		validator, err := newTLSV3ValidationContext(
			tb, "",
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS validation context")
		}
		resources = append(resources, validator)
	case names[envoyDefaultBundleName]:
		// If they've requested the default bundle, we send the connected trust
		// domain's bundle - but - we override the name to match what they
		// expect.
		validator, err := newTLSV3ValidationContext(
			tb, envoyDefaultBundleName,
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS validation context")
		}
		resources = append(resources, validator)
	case names[envoyAllBundlesName]:
		// TODO: When federation support is added, the behavior of this case
		// shall change to return the connected trust domain's bundle
		// concatenated with all federated bundles. For now, we return the
		// connected trust domain's bundle - but - we override the name to
		// match what they expect.
		validator, err := newTLSV3ValidationContext(
			tb, envoyAllBundlesName,
		)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS validation context")
		}
		resources = append(resources, validator)
	}

	// TODO: When federation support is added, return any federated bundles
	// if named or returnAll.

	return &discoveryv3pb.DiscoveryResponse{
		// Copy in requested type from req
		TypeUrl:     req.TypeUrl,
		VersionInfo: "",
		Resources:   resources,
	}, nil
}

// StreamSecrets implements
// envoy.service.secret.v3/SecretDiscoveryService.StreamSecrets.
// This should return the current SVIDs and CAs, and stream any future updates.
func (s *SPIFFEWorkloadAPIService) StreamSecrets(
	srv secretv3pb.SecretDiscoveryService_StreamSecretsServer,
) error {
	// TODO: Implement.
	return status.Error(
		codes.Unimplemented,
		"method not implemented",
	)
}

// DeltaSecrets implements
// envoy.service.secret.v3/SecretDiscoveryService.DeltaSecrets.
// We do not implement this method.
func (s *SPIFFEWorkloadAPIService) DeltaSecrets(
	_ secretv3pb.SecretDiscoveryService_DeltaSecretsServer,
) error {
	return status.Error(
		codes.Unimplemented,
		"method not implemented",
	)
}
