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
	"fmt"

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

	"github.com/gravitational/teleport/lib/tbot/config"
)

// Various code related to implementation of Envoy SDS API
//
// This effectively replaces the Workload API for Envoy, but functions in a
// very similar way.
//
// For now, we declare support for 1.18 and above since this version introduced
// the SPIFFE-specific validator.

func newTLSV3Certificate(svid *workloadpb.X509SVID) (*anyv1pb.Any, error) {
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
	return anypb.New(&tlsv3pb.Secret{
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
	})
}

const envoySPIFFECertValidator = "envoy.tls.cert_validator.spiffe"

func newTLSV3ValidationContext(tb *x509bundle.Bundle) (*anyv1pb.Any, error) {
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
	return anypb.New(&tlsv3pb.Secret{
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
	})
}

// FetchSecrets implements
// envoy.service.secret.v3/SecretDiscoveryService.FetchSecrets.
// This should return the current SVIDs and trust bundles.
func (s *SPIFFEWorkloadAPIService) FetchSecrets(
	ctx context.Context,
	req *discoveryv3pb.DiscoveryRequest,
) (*discoveryv3pb.DiscoveryResponse, error) {
	log, creds, err := s.authenticateClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "authenticating client")
	}

	log.InfoContext(ctx, "SecretDiscoveryService.FetchSecrets request received from workload")
	defer log.InfoContext(ctx, "SecretDiscoveryService.FetchSecrets request handled")

	// names holds all names requested by the client
	// if this is nothing, we assume they want everything available to them.
	// it's worth keeping in mind that there's some special names we need to
	// handle. TODO: handle default names
	names := map[string]bool{}
	for _, name := range req.ResourceNames {
		// Ignore empty string
		if name != "" {
			names[name] = true
		}
	}
	wantAll := len(names) == 0

	var resources []*anyv1pb.Any

	// Filter SVIDs down to those accessible to this workload, then filter down
	// to those request by the client.
	availableSVIDs := filterSVIDRequests(ctx, log, s.cfg.SVIDs, creds)
	wantedSVIDs := make([]config.SVIDRequest, 0)
	for _, svidReq := range availableSVIDs {
		// TODO: Make this comparison cleaner...
		// TODO: Handle default name.
		if wantAll || names[fmt.Sprintf("spiffe://%s%s", s.trustDomain, svidReq.Path)] {
			wantedSVIDs = append(wantedSVIDs, svidReq)
		}
	}
	// Fetch the SVIDs and convert them into the SDS cert type.
	svids, err := s.fetchX509SVIDs(ctx, log, wantedSVIDs)
	if err != nil {
		return nil, trace.Wrap(err, "fetching X509 SVIDs")
	}
	for _, svid := range svids {
		// TODO: Handle default name
		secret, err := newTLSV3Certificate(svid)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS certificate")
		}
		resources = append(resources, secret)
	}

	// Convert trust bundle to SDS validator type.
	// TODO: Federation support!!
	// TODO: Handle default name
	if wantAll || names[s.trustBundle.TrustDomain().IDString()] {
		validator, err := newTLSV3ValidationContext(s.trustBundle)
		if err != nil {
			return nil, trace.Wrap(err, "creating TLS validation context")
		}
		resources = append(resources, validator)
	}

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
	_ secretv3pb.SecretDiscoveryService_StreamSecretsServer,
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
func (s *SPIFFEWorkloadAPIService) DeltaSecrets(_ secretv3pb.SecretDiscoveryService_DeltaSecretsServer) error {
	return status.Error(
		codes.Unimplemented,
		"method not implemented",
	)
}
