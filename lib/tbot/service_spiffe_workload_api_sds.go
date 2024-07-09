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
	workloadpb "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
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

func newTLSV3ValidationContext(trustDomain string) (*anyv1pb.Any, error) {
	// TODO: Federation support!
	trustDomains := []*tlsv3pb.SPIFFECertValidatorConfig_TrustDomain{
		{
			Name: "example.teleport.sh", // TODO: Use real trust domain
			TrustBundle: &corev3pb.DataSource{
				Specifier: &corev3pb.DataSource_InlineBytes{
					// Must be PEM-wrapped X509 certificates
					InlineBytes: []byte(nil), // TODO: Add trust bundle
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
		// TODO: WHat is name??
		Name: name,
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
	log, _, err := s.authenticateClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "authenticating client")
	}

	log.InfoContext(ctx, "SecretDiscoveryService.FetchSecrets request received from workload")
	defer log.InfoContext(ctx, "SecretDiscoveryService.FetchSecrets request handled")

	resources := []*anyv1pb.Any{}

	// Fetch SVIDs and convert

	// Filter down to requested SVID - if none requested or none match, return
	// all.

	// Fetch trust bundles and convert
	s.getTrustBundle()

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
