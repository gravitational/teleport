// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package tbot

import (
	"bytes"
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

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
)

// WorkloadIdentityX509Service is a service that retrieves X.509 certificates
// for WorkloadIdentity resources.
type WorkloadIdentityX509Service struct {
	botAuthClient  *authclient.Client
	botCfg         *config.BotConfig
	cfg            *config.WorkloadIdentityX509Service
	getBotIdentity getBotIdentityFn
	log            *slog.Logger
	resolver       reversetunnelclient.Resolver
	// trustBundleCache is the cache of trust bundles. It only needs to be
	// provided when running in daemon mode.
	trustBundleCache *workloadidentity.TrustBundleCache
}

// String returns a human-readable description of the service.
func (s *WorkloadIdentityX509Service) String() string {
	return fmt.Sprintf("workload-identity-x509 (%s)", s.cfg.Destination.String())
}

// OneShot runs the service once, generating the output and writing it to the
// destination, before exiting.
func (s *WorkloadIdentityX509Service) OneShot(ctx context.Context) error {
	res, privateKey, err := s.requestSVID(ctx)
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
	return s.render(ctx, bundleSet, res, privateKey)
}

// Run runs the service in daemon mode, periodically generating the output and
// writing it to the destination.
func (s *WorkloadIdentityX509Service) Run(ctx context.Context) error {
	bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return trace.Wrap(err, "getting trust bundle set")
	}

	jitter := retryutils.DefaultJitter
	var x509Cred *workloadidentityv1pb.Credential
	var privateKey crypto.Signer
	var failures int
	firstRun := make(chan struct{}, 1)
	firstRun <- struct{}{}
	for {
		var retryAfter <-chan time.Time
		if failures > 0 {
			backoffTime := time.Second * time.Duration(math.Pow(2, float64(failures-1)))
			if backoffTime > time.Minute {
				backoffTime = time.Minute
			}
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
				x509Cred = nil
				privateKey = nil
			}
			bundleSet = newBundleSet
		case <-time.After(cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).RenewalInterval):
			s.log.InfoContext(ctx, "Renewal interval reached, renewing SVIDs")
			x509Cred = nil
			privateKey = nil
		case <-firstRun:
		}

		if x509Cred == nil || privateKey == nil {
			var err error
			x509Cred, privateKey, err = s.requestSVID(ctx)
			if err != nil {
				s.log.ErrorContext(ctx, "Failed to request SVID", "error", err)
				failures++
				continue
			}
		}
		if err := s.render(ctx, bundleSet, x509Cred, privateKey); err != nil {
			s.log.ErrorContext(ctx, "Failed to render output", "error", err)
			failures++
			continue
		}
		failures = 0
	}
}

func (s *WorkloadIdentityX509Service) requestSVID(
	ctx context.Context,
) (
	*workloadidentityv1pb.Credential,
	crypto.Signer,
	error,
) {
	ctx, span := tracer.Start(
		ctx,
		"WorkloadIdentityX509Service/requestSVID",
	)
	defer span.End()

	roles, err := fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
	if err != nil {
		return nil, nil, trace.Wrap(err, "fetching roles")
	}

	id, err := generateIdentity(
		ctx,
		s.botAuthClient,
		s.getBotIdentity(),
		roles,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
		nil,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	facade := identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, id)
	impersonatedClient, err := clientForFacade(ctx, s.log, s.botCfg, facade, s.resolver)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	x509Credentials, privateKey, err := workloadidentity.IssueX509WorkloadIdentity(
		ctx,
		s.log,
		impersonatedClient,
		s.cfg.Selector,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
		nil,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating X509 SVID")
	}
	var x509Credential *workloadidentityv1pb.Credential
	switch len(x509Credentials) {
	case 0:
		return nil, nil, trace.BadParameter("no X509 SVIDs returned")
	case 1:
		x509Credential = x509Credentials[0]
	default:
		// We could eventually implement some kind of hint selection mechanism
		// to pick the "right" one.
		received := make([]string, 0, len(x509Credentials))
		for _, cred := range x509Credentials {
			received = append(received,
				fmt.Sprintf(
					"%s:%s",
					cred.WorkloadIdentityName,
					cred.SpiffeId,
				),
			)
		}
		return nil, nil, trace.BadParameter(
			"multiple X509 SVIDs received: %v", received,
		)
	}

	return x509Credential, privateKey, nil
}

func (s *WorkloadIdentityX509Service) render(
	ctx context.Context,
	bundleSet *workloadidentity.BundleSet,
	x509Cred *workloadidentityv1pb.Credential,
	privateKey crypto.Signer,
) error {
	ctx, span := tracer.Start(
		ctx,
		"WorkloadIdentityX509Service/render",
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
		Type:  pemPrivateKey,
		Bytes: privBytes,
	})

	if err := s.cfg.Destination.Write(ctx, config.SVIDKeyPEMPath, privPEM); err != nil {
		return trace.Wrap(err, "writing svid key")
	}

	var certPEM bytes.Buffer
	pem.Encode(&certPEM, &pem.Block{
		Type:  pemCertificate,
		Bytes: x509Cred.GetX509Svid().GetCert(),
	})
	for _, c := range x509Cred.GetX509Svid().GetChain() {
		pem.Encode(&certPEM, &pem.Block{
			Type:  pemCertificate,
			Bytes: c,
		})
	}
	if err := s.cfg.Destination.Write(ctx, config.SVIDPEMPath, certPEM.Bytes()); err != nil {
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
		ctx, config.SVIDTrustBundlePEMPath, trustBundleBytes,
	); err != nil {
		return trace.Wrap(err, "writing svid trust bundle")
	}

	s.log.InfoContext(
		ctx,
		"Successfully wrote X509 workload identity credential to destination",
		"workload_identity", workloadidentity.WorkloadIdentityLogValue(x509Cred),
		"destination", s.cfg.Destination.String(),
	)
	return nil
}
