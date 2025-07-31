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

package legacyspiffe

import (
	"cmp"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/durationpb"

	apiclient "github.com/gravitational/teleport/api/client"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
)

func SVIDOutputServiceBuilder(
	cfg *SVIDOutputConfig,
	trustBundleCache TrustBundleGetter,
	defaultCredentialLifetime bot.CredentialLifetime,
) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &SVIDOutputService{
			botAuthClient:             deps.Client,
			defaultCredentialLifetime: defaultCredentialLifetime,
			cfg:                       cfg,
			getBotIdentity:            deps.BotIdentity,
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
		}
		svc.log = deps.LoggerForService(svc)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())
		return svc, nil
	}
}

// SVIDOutputService is a service that generates and writes X509 SPIFFE
// SVIDs to a destination. It produces an output compatible with the
// `spiffe-helper` tool.
type SVIDOutputService struct {
	botAuthClient             *apiclient.Client
	defaultCredentialLifetime bot.CredentialLifetime
	cfg                       *SVIDOutputConfig
	getBotIdentity            func() *identity.Identity
	log                       *slog.Logger
	statusReporter            readyz.Reporter
	// trustBundleCache is the cache of trust bundles. It only needs to be
	// provided when running in daemon mode.
	trustBundleCache  TrustBundleGetter
	identityGenerator *identity.Generator
	clientBuilder     *client.Builder
}

func (s *SVIDOutputService) String() string {
	return cmp.Or(
		s.cfg.Name,
		fmt.Sprintf("spiffe-svid-output (%s)", s.cfg.Destination.String()),
	)
}

func (s *SVIDOutputService) OneShot(ctx context.Context) error {
	res, privateKey, jwtSVIDs, err := s.requestSVID(ctx)
	if err != nil {
		return trace.Wrap(err, "requesting SVID")
	}
	bundleSet, err := workloadidentity.FetchInitialBundleSet(
		ctx,
		s.log,
		s.botAuthClient.SPIFFEFederationServiceClient(),
		s.botAuthClient.TrustClient(),
		s.cfg.IncludeFederatedTrustBundles,
		s.getBotIdentity().ClusterName,
	)
	if err != nil {
		return trace.Wrap(err, "fetching trust bundle set")

	}
	return s.render(ctx, bundleSet, res, privateKey, jwtSVIDs)
}

func (s *SVIDOutputService) Run(ctx context.Context) error {
	bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return trace.Wrap(err, "getting trust bundle set")
	}

	jitter := retryutils.DefaultJitter
	var res *machineidv1pb.SignX509SVIDsResponse
	var privateKey crypto.Signer
	var jwtSVIDs map[string]string
	var failures int
	firstRun := make(chan struct{}, 1)
	firstRun <- struct{}{}
	for {
		var retryAfter <-chan time.Time
		if failures > 0 {
			s.statusReporter.Report(readyz.Unhealthy)
			backoffTime := min(time.Second*time.Duration(math.Pow(2, float64(failures-1))), time.Minute)
			backoffTime = jitter(backoffTime)
			s.log.WarnContext(
				ctx,
				"Last attempt to generate output failed, will retry",
				"retry_after", backoffTime,
				"failures", failures,
			)
			retryAfter = time.After(time.Duration(failures) * time.Second)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-retryAfter:
			s.log.InfoContext(ctx, "Retrying")
		case <-bundleSet.Stale():
			newBundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
			if err != nil {
				return trace.Wrap(err, "getting trust bundle set")
			}
			s.log.InfoContext(ctx, "Trust bundle set has been updated")
			if !newBundleSet.Local.Equal(bundleSet.Local) {
				// If the local trust domain CA has changed, we need to reissue
				// the SVID.
				res = nil
				privateKey = nil
			}
			bundleSet = newBundleSet
		case <-time.After(cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).RenewalInterval):
			s.log.InfoContext(ctx, "Renewal interval reached, renewing SVIDs")
			res = nil
			privateKey = nil
		case <-firstRun:
		}

		if res == nil || privateKey == nil {
			var err error
			res, privateKey, jwtSVIDs, err = s.requestSVID(ctx)
			if err != nil {
				s.log.ErrorContext(ctx, "Failed to request SVID", "error", err)
				failures++
				continue
			}
		}
		if err := s.render(ctx, bundleSet, res, privateKey, jwtSVIDs); err != nil {
			s.log.ErrorContext(ctx, "Failed to render output", "error", err)
			failures++
			continue
		}
		s.statusReporter.Report(readyz.Healthy)
		failures = 0
	}
}

func (s *SVIDOutputService) requestSVID(
	ctx context.Context,
) (
	*machineidv1pb.SignX509SVIDsResponse,
	crypto.Signer,
	map[string]string,
	error,
) {
	ctx, span := tracer.Start(
		ctx,
		"SVIDOutputService/requestSVID",
	)
	defer span.End()

	effectiveLifetime := cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime)
	id, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "generating identity")
	}

	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	impersonatedClient, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	res, privateKey, err := generateSVID(
		ctx,
		impersonatedClient,
		[]SVIDRequest{s.cfg.SVID},
		cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).TTL,
	)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "generating X509 SVID")
	}

	jwtSvids, err := generateJWTSVIDs(
		ctx,
		impersonatedClient,
		s.cfg.SVID,
		s.cfg.JWTs,
		cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime).TTL)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "generating JWT SVIDs")
	}

	return res, privateKey, jwtSvids, nil
}

func (s *SVIDOutputService) render(
	ctx context.Context,
	bundleSet *workloadidentity.BundleSet,
	res *machineidv1pb.SignX509SVIDsResponse,
	privateKey crypto.Signer,
	jwtSVIDs map[string]string,
) error {
	ctx, span := tracer.Start(
		ctx,
		"SVIDOutputService/render",
	)
	defer span.End()
	s.log.InfoContext(ctx, "Rendering output")

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

	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return trace.Wrap(err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  internal.PEMBlockTypePrivateKey,
		Bytes: privBytes,
	})

	if len(res.Svids) != 1 {
		return trace.BadParameter("expected 1 SVID, got %d", len(res.Svids))

	}
	svid := res.Svids[0]
	if err := s.cfg.Destination.Write(ctx, internal.SVIDKeyPEMPath, privPEM); err != nil {
		return trace.Wrap(err, "writing svid key")
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  internal.PEMBlockTypeCertificate,
		Bytes: svid.Certificate,
	})
	if err := s.cfg.Destination.Write(ctx, internal.SVIDPEMPath, certPEM); err != nil {
		return trace.Wrap(err, "writing svid certificate")
	}

	trustBundleBytes, err := bundleSet.Local.X509Bundle().Marshal()
	if err != nil {
		return trace.Wrap(err, "marshaling local trust bundle")
	}

	if s.cfg.IncludeFederatedTrustBundles {
		for _, federatedBundle := range bundleSet.Federated {
			federatedBundleBytes, err := federatedBundle.X509Bundle().Marshal()
			if err != nil {
				return trace.Wrap(err, "marshaling federated trust bundle (%s)", federatedBundle.TrustDomain().Name())
			}
			trustBundleBytes = append(trustBundleBytes, federatedBundleBytes...)
		}
	}

	if err := s.cfg.Destination.Write(
		ctx, internal.SVIDTrustBundlePEMPath, trustBundleBytes,
	); err != nil {
		return trace.Wrap(err, "writing svid trust bundle")
	}

	for fileName, jwt := range jwtSVIDs {
		if err := s.cfg.Destination.Write(ctx, fileName, []byte(jwt)); err != nil {
			return trace.Wrap(err, "writing JWT SVID")
		}
	}

	return nil
}

func generateJWTSVIDs(
	ctx context.Context,
	clt *apiclient.Client,
	svid SVIDRequest,
	reqs []JWTSVID,
	ttl time.Duration,
) (map[string]string, error) {
	ctx, span := tracer.Start(
		ctx,
		"generateJWTSVIDs",
	)
	defer span.End()

	requestedAudiences := map[string]bool{}
	for _, jwt := range reqs {
		requestedAudiences[jwt.Audience] = true
	}

	jwtReqs := make([]*machineidv1pb.JWTSVIDRequest, 0, len(requestedAudiences))
	for audience := range requestedAudiences {
		jwtReqs = append(jwtReqs, &machineidv1pb.JWTSVIDRequest{
			Audiences:    []string{audience},
			Ttl:          durationpb.New(ttl),
			SpiffeIdPath: svid.Path,
		})
	}

	if len(jwtReqs) == 0 {
		return nil, nil
	}

	jwtRes, err := clt.WorkloadIdentityServiceClient().SignJWTSVIDs(ctx, &machineidv1pb.SignJWTSVIDsRequest{
		Svids: jwtReqs,
	})
	if err != nil {
		return nil, trace.Wrap(err, "requesting JWT SVIDs")
	}

	jwtFiles := map[string]string{}
	for _, req := range reqs {
		for _, jwtSVID := range jwtRes.Svids {
			if len(jwtSVID.Audiences) == 1 && jwtSVID.Audiences[0] == req.Audience {
				jwtFiles[req.FileName] = jwtSVID.Jwt
				break
			}
		}
	}
	return jwtFiles, nil
}

// generateSVID generates the pre-requisites and makes a SVID generation RPC
// call.
func generateSVID(
	ctx context.Context,
	clt *apiclient.Client,
	reqs []SVIDRequest,
	ttl time.Duration,
) (*machineidv1pb.SignX509SVIDsResponse, crypto.Signer, error) {
	ctx, span := tracer.Start(
		ctx,
		"generateSVID",
	)
	defer span.End()
	privateKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(clt),
		cryptosuites.BotSVID)
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
