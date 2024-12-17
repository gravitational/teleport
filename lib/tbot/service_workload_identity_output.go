package tbot

import (
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

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/spiffe"
)

// WorkloadIdentityOutputService
type WorkloadIdentityOutputService struct {
	botAuthClient  *authclient.Client
	botCfg         *config.BotConfig
	cfg            *config.WorkloadIdentityOutput
	getBotIdentity getBotIdentityFn
	log            *slog.Logger
	resolver       reversetunnelclient.Resolver
	// trustBundleCache is the cache of trust bundles. It only needs to be
	// provided when running in daemon mode.
	trustBundleCache *spiffe.TrustBundleCache
}

func (s *WorkloadIdentityOutputService) String() string {
	return fmt.Sprintf("spiffe-svid-output (%s)", s.cfg.Destination.String())
}

func (s *WorkloadIdentityOutputService) OneShot(ctx context.Context) error {
	res, privateKey, jwtSVIDs, err := s.requestSVID(ctx)
	if err != nil {
		return trace.Wrap(err, "requesting SVID")
	}
	bundleSet, err := spiffe.FetchInitialBundleSet(
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

func (s *WorkloadIdentityOutputService) Run(ctx context.Context) error {
	bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return trace.Wrap(err, "getting trust bundle set")
	}

	jitter := retryutils.DefaultJitter
	var x509Cred *workloadidentityv1pb.Credential
	var privateKey crypto.Signer
	var jwtCred *workloadidentityv1pb.Credential
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
				jwtCred = nil
				privateKey = nil
			}
			bundleSet = newBundleSet
		case <-time.After(s.botCfg.RenewalInterval):
			s.log.InfoContext(ctx, "Renewal interval reached, renewing SVIDs")
			x509Cred = nil
			jwtCred = nil
			privateKey = nil
		case <-firstRun:
		}

		if x509Cred == nil || privateKey == nil {
			var err error
			x509Cred, privateKey, jwtCred, err = s.requestSVID(ctx)
			if err != nil {
				s.log.ErrorContext(ctx, "Failed to request SVID", "error", err)
				failures++
				continue
			}
		}
		if err := s.render(ctx, bundleSet, x509Cred, privateKey, jwtCred); err != nil {
			s.log.ErrorContext(ctx, "Failed to render output", "error", err)
			failures++
			continue
		}
		failures = 0
	}
}

func (s *WorkloadIdentityOutputService) requestSVID(
	ctx context.Context,
) (
	*workloadidentityv1pb.Credential,
	crypto.Signer,
	*workloadidentityv1pb.Credential,
	error,
) {
	ctx, span := tracer.Start(
		ctx,
		"WorkloadIdentityOutputService/requestSVID",
	)
	defer span.End()

	roles, err := fetchDefaultRoles(ctx, s.botAuthClient, s.getBotIdentity())
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "fetching roles")
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
		return nil, nil, nil, trace.Wrap(err, "generating identity")
	}
	// create a client that uses the impersonated identity, so that when we
	// fetch information, we can ensure access rights are enforced.
	facade := identity.NewFacade(s.botCfg.FIPS, s.botCfg.Insecure, id)
	impersonatedClient, err := clientForFacade(ctx, s.log, s.botCfg, facade, s.resolver)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	defer impersonatedClient.Close()

	x509Credentials, privateKey, err := issueX509WorkloadIdentity(
		ctx,
		impersonatedClient,
		s.cfg.WorkloadIdentity,
		s.botCfg.CertificateTTL,
		nil,
	)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "generating X509 SVID")
	}
	var x509Credential *workloadidentityv1pb.Credential
	switch len(x509Credentials) {
	case 0:
		return nil, nil, nil, trace.BadParameter("no X509 SVIDs returned")
	case 1:
		x509Credential = x509Credentials[0]
	default:
		// We could eventually implement some kind of hint selection mechanism
		// to pick the "right" one.
		return nil, nil, nil, trace.BadParameter("multiple X509 SVIDs received")
	}

	jwtSvid, err := issueJWTWorkloadIdentity(
		ctx,
		impersonatedClient,
		s.cfg.WorkloadIdentity,
		s.cfg.JWTAudiences,
		s.botCfg.CertificateTTL,
		nil,
	)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "generating JWT SVIDs")
	}
	var jwtCred *workloadidentityv1pb.Credential
	switch len(jwtSvid) {
	case 0:
		// No JWT SVIDs were requested, so we don't need to do anything.
	case 1:
		jwtCred = jwtSvid[0]
	default:
		// We could eventually implement some kind of hint selection mechanism
		// to pick the "right" one.
		return nil, nil, nil, trace.BadParameter("multiple JWT SVIDs received")
	}

	return x509Credential, privateKey, jwtCred, nil
}

func (s *WorkloadIdentityOutputService) render(
	ctx context.Context,
	bundleSet *spiffe.BundleSet,
	x509Cred *workloadidentityv1pb.Credential,
	privateKey crypto.Signer,
	jwtCred *workloadidentityv1pb.Credential,
) error {
	ctx, span := tracer.Start(
		ctx,
		"WorkloadIdentityOutputService/render",
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

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  pemCertificate,
		Bytes: x509Cred.GetX509Svid().GetCert(),
	})
	if err := s.cfg.Destination.Write(ctx, config.SVIDPEMPath, certPEM); err != nil {
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

	if jwtCred != nil {
		if err := s.cfg.Destination.Write(ctx, "jwt", []byte(jwtCred.GetJwtSvid().GetJwt())); err != nil {
			return trace.Wrap(err, "writing JWT SVID")
		}
	}

	return nil
}

func issueX509WorkloadIdentity(
	ctx context.Context,
	clt *authclient.Client,
	workloadIdentity config.WorkloadIdentitySelector,
	ttl time.Duration,
	attest *workloadidentityv1pb.WorkloadAttrs,
) ([]*workloadidentityv1pb.Credential, crypto.Signer, error) {
	ctx, span := tracer.Start(
		ctx,
		"issueX509WorkloadIdentity",
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

	// When using the "name" based selector, we either get a single WIC back,
	// or an error. We don't need to worry about selecting the right one.
	res, err := clt.WorkloadIdentityIssuanceClient().IssueWorkloadIdentity(ctx,
		&workloadidentityv1pb.IssueWorkloadIdentityRequest{
			Name: workloadIdentity.Name,
			Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
				X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
					PublicKey: pubBytes,
				},
			},
			RequestedTtl:  durationpb.New(ttl),
			WorkloadAttrs: attest,
		},
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// TODO: Log intimate details of the issued credential

	return []*workloadidentityv1pb.Credential{res.Credential}, privateKey, nil
}

func issueJWTWorkloadIdentity(
	ctx context.Context,
	clt *authclient.Client,
	workloadIdentity config.WorkloadIdentitySelector,
	audiences []string,
	ttl time.Duration,
	attest *workloadidentityv1pb.WorkloadAttrs,
) ([]*workloadidentityv1pb.Credential, error) {
	ctx, span := tracer.Start(
		ctx,
		"issueJWTWorkloadIdentity",
	)
	defer span.End()

	if len(audiences) == 0 {
		return nil, nil
	}

	// When using the "name" based selector, we either get a single WIC back,
	// or an error. We don't need to worry about selecting the right one.
	res, err := clt.WorkloadIdentityIssuanceClient().IssueWorkloadIdentity(ctx,
		&workloadidentityv1pb.IssueWorkloadIdentityRequest{
			Name: workloadIdentity.Name,
			Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
				JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
					Audiences: audiences,
				},
			},
			RequestedTtl:  durationpb.New(ttl),
			WorkloadAttrs: attest,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: Log intimate details of the issued credential

	return []*workloadidentityv1pb.Credential{res.Credential}, nil
}
