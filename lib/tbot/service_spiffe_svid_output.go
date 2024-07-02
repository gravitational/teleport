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
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const (
	// pemPrivateKey is the PEM block type for a PKCS 8 encoded private key.
	pemPrivateKey = "PRIVATE KEY"
	// pemCertificate is the PEM block type for a DER encoded certificate.
	pemCertificate = "CERTIFICATE"
)

// SPIFFESVIDOutputService
type SPIFFESVIDOutputService struct {
	botAuthClient     *authclient.Client
	botCfg            *config.BotConfig
	cfg               *config.SPIFFESVIDOutput
	getBotIdentity    getBotIdentityFn
	log               *slog.Logger
	reloadBroadcaster *channelBroadcaster
	resolver          reversetunnelclient.Resolver
}

func (s *SPIFFESVIDOutputService) String() string {
	return fmt.Sprintf("spiffe-svid-output (%s)", s.cfg.Destination.String())
}

func (s *SPIFFESVIDOutputService) OneShot(ctx context.Context) error {
	return s.generate(ctx)
}

func (s *SPIFFESVIDOutputService) Run(ctx context.Context) error {
	reloadCh, unsubscribe := s.reloadBroadcaster.subscribe()
	defer unsubscribe()

	err := runOnInterval(ctx, runOnIntervalConfig{
		name:       "output-renewal",
		f:          s.generate,
		interval:   s.botCfg.RenewalInterval,
		retryLimit: renewalRetryLimit,
		log:        s.log,
		reloadCh:   reloadCh,
	})
	return trace.Wrap(err)
}

func (s *SPIFFESVIDOutputService) generate(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"SPIFFESVIDOutputService/generate",
	)
	defer span.End()
	s.log.InfoContext(ctx, "Generating output")

	// Check the ACLs. We can't fix them, but we can warn if they're
	// misconfigured. We'll need to precompute a list of keys to check.
	// Note: This may only log a warning, depending on configuration.
	if err := s.cfg.Destination.Verify(identity.ListKeys(identity.DestinationKinds()...)); err != nil {
		return trace.Wrap(err)
	}
	// Ensure this destination is also writable. This is a hard fail if
	// ACLs are misconfigured, regardless of configuration.
	if err := identity.VerifyWrite(ctx, s.cfg.Destination); err != nil {
		return trace.Wrap(err, "verifying destination")
	}

	roles, err := fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
	if err != nil {
		return trace.Wrap(err, "fetching roles")
	}

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		s.botCfg.CertificateTTL,
		nil,
	)
	if err != nil {
		return trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	facade := identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, id)
	impersonatedClient, err := clientForFacade(ctx, s.log, s.botCfg, facade, s.resolver)
	if err != nil {
		return trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	res, privateKey, err := generateSVID(
		ctx,
		impersonatedClient,
		[]config.SVIDRequest{s.cfg.SVID},
		s.botCfg.CertificateTTL,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return trace.Wrap(err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  pemPrivateKey,
		Bytes: privBytes,
	})

	spiffeCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.SPIFFECA, false)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(res.Svids) != 1 {
		return trace.BadParameter("expected 1 SVID, got %d", len(res.Svids))

	}
	svid := res.Svids[0]
	if err := s.cfg.Destination.Write(ctx, config.SVIDKeyPEMPath, privPEM); err != nil {
		return trace.Wrap(err, "writing svid key")
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  pemCertificate,
		Bytes: svid.Certificate,
	})
	if err := s.cfg.Destination.Write(ctx, config.SVIDPEMPath, certPEM); err != nil {
		return trace.Wrap(err, "writing svid certificate")
	}

	trustBundleBytes := &bytes.Buffer{}
	for _, ca := range spiffeCAs {
		for _, cert := range services.GetTLSCerts(ca) {
			// Values are already PEM encoded, so we just append to the buffer
			if _, err := trustBundleBytes.Write(cert); err != nil {
				return trace.Wrap(err, "writing trust bundle to buffer")
			}
		}
	}
	if err := s.cfg.Destination.Write(
		ctx, config.SVIDTrustBundlePEMPath, trustBundleBytes.Bytes(),
	); err != nil {
		return trace.Wrap(err, "writing svid trust bundle")
	}

	return nil
}

// generateSVID generates the pre-requisites and makes a SVID generation RPC
// call.
func generateSVID(
	ctx context.Context,
	clt *authclient.Client,
	reqs []config.SVIDRequest,
	ttl time.Duration,
) (*machineidv1pb.SignX509SVIDsResponse, *rsa.PrivateKey, error) {
	ctx, span := tracer.Start(
		ctx,
		"generateSVID",
	)
	defer span.End()
	privateKey, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	svids := make([]*machineidv1pb.SVIDRequest, 0, len(reqs))
	for _, req := range reqs {
		svids = append(svids, &machineidv1pb.SVIDRequest{
			PublicKey:    pubBytes,
			SpiffeIdPath: req.Path,
			DnsSans:      req.SANS.DNS,
			IpSans:       req.SANS.IP,
			Hint:         req.Hint,
			Ttl:          durationpb.New(ttl),
		})
	}

	res, err := clt.WorkloadIdentityServiceClient().SignX509SVIDs(ctx,
		&machineidv1pb.SignX509SVIDsRequest{
			Svids: svids,
		},
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return res, privateKey, nil
}
