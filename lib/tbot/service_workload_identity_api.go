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
	"cmp"
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"time"

	secretv3pb "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/gravitational/trace"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	workloadpb "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
	"github.com/gravitational/teleport/lib/uds"
)

// WorkloadIdentityAPIService implements a gRPC server that fulfills the SPIFFE
// Workload API specification. It provides X509 SVIDs and trust bundles to
// workloads that connect over the configured listener.
//
// Sources:
// - https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_Endpoint.md
// - https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_API.md
// - https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md
// - https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md
type WorkloadIdentityAPIService struct {
	workloadpb.UnimplementedSpiffeWorkloadAPIServer

	svcIdentity      *config.UnstableClientCredentialOutput
	botCfg           *config.BotConfig
	cfg              *config.WorkloadIdentityAPIService
	log              *slog.Logger
	resolver         reversetunnelclient.Resolver
	trustBundleCache *workloadidentity.TrustBundleCache
	crlCache         *workloadidentity.CRLCache

	// client holds the impersonated client for the service
	client           *authclient.Client
	attestor         *workloadattest.Attestor
	localTrustDomain spiffeid.TrustDomain
}

// setup initializes the service, performing tasks such as determining the
// trust domain, fetching the initial trust bundle and creating an impersonated
// client.
func (s *WorkloadIdentityAPIService) setup(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "WorkloadIdentityAPIService/setup")
	defer span.End()

	// Wait for the impersonated identity to be ready for us to consume here.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return trace.BadParameter("timeout waiting for identity to be ready")
	case <-s.svcIdentity.Ready():
	}
	facade, err := s.svcIdentity.Facade()
	if err != nil {
		return trace.Wrap(err)
	}
	client, err := clientForFacade(
		ctx, s.log, s.botCfg, facade, s.resolver,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	s.client = client
	// Closure is managed by the caller if this function succeeds. But if it
	// fails, we need to close the client.
	defer func() {
		if err != nil {
			client.Close()
		}
	}()

	td, err := spiffeid.TrustDomainFromString(facade.Get().ClusterName)
	if err != nil {
		return trace.Wrap(err, "parsing trust domain name")
	}
	s.localTrustDomain = td

	s.attestor, err = workloadattest.NewAttestor(s.log, s.cfg.Attestors)
	if err != nil {
		return trace.Wrap(err, "setting up workload attestation")
	}

	return nil
}

func (s *WorkloadIdentityAPIService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "WorkloadIdentityAPIService/Run")
	defer span.End()

	s.log.DebugContext(ctx, "Starting pre-run initialization")
	if err := s.setup(ctx); err != nil {
		return trace.Wrap(err)
	}
	defer s.client.Close()
	s.log.DebugContext(ctx, "Completed pre-run initialization")

	srvMetrics := metrics.CreateGRPCServerMetrics(
		true, prometheus.Labels{
			teleport.TagServer: "tbot-workload-identity-api",
		},
	)
	if err := metrics.RegisterPrometheusCollectors(srvMetrics); err != nil {
		return trace.Wrap(err)
	}
	srv := grpc.NewServer(
		grpc.Creds(
			// SPEC (SPIFFE_Workload_endpoint) 3. Transport:
			// - Transport Layer Security MUST NOT be required
			// TODO(noah): We should optionally provide TLS support here down
			// the road.
			uds.NewTransportCredentials(insecure.NewCredentials()),
		),
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(),
			srvMetrics.UnaryServerInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			recovery.StreamServerInterceptor(),
			srvMetrics.StreamServerInterceptor(),
		),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
	)
	workloadpb.RegisterSpiffeWorkloadAPIServer(srv, s)
	sdsHandler := &spiffeSDSHandler{
		log:              s.log,
		botCfg:           s.botCfg,
		trustBundleCache: s.trustBundleCache,
		clientAuthenticator: func(ctx context.Context) (*slog.Logger, svidFetcher, error) {
			log, attrs, err := s.authenticateClient(ctx)
			if err != nil {
				return log, nil, trace.Wrap(err, "authenticating client")
			}

			fetchSVIDs := func(
				ctx context.Context,
				localBundle *spiffebundle.Bundle,
			) ([]*workloadpb.X509SVID, error) {
				return s.fetchX509SVIDs(ctx, log, localBundle, attrs)
			}

			return log, fetchSVIDs, nil
		},
	}
	secretv3pb.RegisterSecretDiscoveryServiceServer(srv, sdsHandler)

	lis, err := createListener(ctx, s.log, s.cfg.Listen)
	if err != nil {
		return trace.Wrap(err, "creating listener")
	}
	defer func() {
		if err := lis.Close(); err != nil {
			s.log.ErrorContext(ctx, "Encountered error closing listener", "error", err)
		}
	}()
	s.log.InfoContext(ctx, "Listener opened for Workload API endpoint", "addr", lis.Addr().String())
	if lis.Addr().Network() == "tcp" {
		s.log.WarnContext(
			ctx, "Workload API endpoint listening on a TCP port. Ensure that only intended hosts can reach this port!",
		)
	}

	// Set off the long running tasks in an errgroup
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		// Start the gRPC server
		return srv.Serve(lis)
	})
	eg.Go(func() error {
		// Shutdown the server when the context is canceled
		<-egCtx.Done()
		s.log.DebugContext(ctx, "Shutting down Workload API endpoint")
		srv.Stop()
		s.log.InfoContext(ctx, "Shut down Workload API endpoint")
		return nil
	})

	return trace.Wrap(eg.Wait())
}

func (s *WorkloadIdentityAPIService) authenticateClient(
	ctx context.Context,
) (*slog.Logger, *workloadidentityv1pb.WorkloadAttrs, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, nil, trace.BadParameter("peer not found in context")
	}
	log := s.log

	if p.Addr.String() != "" {
		log = log.With(
			slog.String("remote_addr", p.Addr.String()),
		)
	}

	authInfo, ok := p.AuthInfo.(uds.AuthInfo)
	// We expect Creds to be nil/unset if the client is connecting via TCP and
	// therefore there is no workload attestation that can be completed.
	if !ok || authInfo.Creds == nil {
		return log, nil, nil
	}

	// For a UDS, sometimes we are unable to determine the PID of the calling
	// workload. This can happen if the caller is calling from another process
	// namespace. In this case, Creds will be non-nil but the PID will be 0.
	//
	// We should fail softly here as there could be SVIDs that do not require
	// workload attestation.
	if authInfo.Creds.PID == 0 {
		log.DebugContext(
			ctx, "Failed to determine the PID of the calling workload. TBot may be running in a different process namespace to the workload. Workload attestation will not be completed.")
		return log, nil, nil
	}

	att, err := s.attestor.Attest(ctx, authInfo.Creds.PID)
	if err != nil {
		// Fail softly as there may be SVIDs configured that don't require any
		// workload attestation and we should still issue those.
		log.ErrorContext(
			ctx,
			"Workload attestation failed",
			"error", err,
			"pid", authInfo.Creds.PID,
		)
		return log, nil, nil
	}
	log = log.With(
		"workload", att,
	)

	return log, att, nil
}

// FetchX509SVID generates and returns the X.509 SVIDs available to a workload.
// It is a streaming RPC, and sends renewed SVIDs to the client before they
// expire.
// Implements the SPIFFE Workload API FetchX509SVID method.
func (s *WorkloadIdentityAPIService) FetchX509SVID(
	_ *workloadpb.X509SVIDRequest,
	srv workloadpb.SpiffeWorkloadAPI_FetchX509SVIDServer,
) error {
	ctx := srv.Context()

	log, creds, err := s.authenticateClient(ctx)
	if err != nil {
		return trace.Wrap(err, "authenticating client")
	}

	log.InfoContext(ctx, "FetchX509SVID stream opened by workload")
	defer log.InfoContext(ctx, "FetchX509SVID stream has closed")

	bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return trace.Wrap(err, "fetching trust bundle set from cache")
	}
	crlSet, err := s.crlCache.GetCRLSet(ctx)
	if err != nil {
		return trace.Wrap(err, "fetching CRL set from cache")
	}

	var svids []*workloadpb.X509SVID
	for {
		log.InfoContext(ctx, "Starting to issue X509 SVIDs to workload")

		// Fetch SVIDs if necessary.
		if svids == nil {
			svids, err = s.fetchX509SVIDs(ctx, log, bundleSet.Local, creds)
			if err != nil {
				return trace.Wrap(err)
			}
			// The SPIFFE Workload API (5.2.1):
			//
			//   If the client is not entitled to receive any X509-SVIDs, then the
			//   server SHOULD respond with the "PermissionDenied" gRPC status code (see
			//   the Error Codes section in the SPIFFE Workload Endpoint specification
			//   for more information). Under such a case, the client MAY attempt to
			//   reconnect with another call to the FetchX509SVID RPC after a backoff.
			if len(svids) == 0 {
				log.ErrorContext(ctx, "Workload did not pass attestation for any SVIDs")
				return status.Error(
					codes.PermissionDenied,
					"workload did not pass attestation for any SVIDs",
				)
			}

		}

		resp := &workloadpb.X509SVIDResponse{
			Svids:            svids,
			FederatedBundles: bundleSet.EncodedX509Bundles(false),
		}
		if len(crlSet.LocalCRL) > 0 {
			// TODO: Copy?
			resp.Crl = [][]byte{crlSet.LocalCRL}
		}

		err = srv.Send(resp)
		if err != nil {
			return trace.Wrap(err)
		}
		log.DebugContext(
			ctx, "Finished issuing SVIDs to workload. Waiting for next renewal interval or CA rotation",
		)

		select {
		case <-ctx.Done():
			log.DebugContext(ctx, "Context closed, stopping SVID stream")
			return nil
		case <-bundleSet.Stale():
			newBundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			log.DebugContext(ctx, "Federated trust bundles have been updated, renewing SVIDs")
			if !newBundleSet.Local.Equal(bundleSet.Local) {
				// If the "local" trust domain's CA has changed, we need to
				// reissue the SVIDs.
				svids = nil
			}
			bundleSet = newBundleSet
			continue
		case <-crlSet.Stale():
			newCRLSet, err := s.crlCache.GetCRLSet(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			log.DebugContext(ctx, "CRL set has been updated, distributing to client")
			crlSet = newCRLSet
			continue
		case <-time.After(cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).RenewalInterval):
			log.DebugContext(ctx, "Renewal interval reached, renewing SVIDs")
			svids = nil
			continue
		}
	}
}

// FetchX509Bundles returns the trust bundle for the trust domain. It is a
// streaming RPC, and will send rotated trust bundles to the client for as long
// as the client is connected.
// Implements the SPIFFE Workload API FetchX509SVID method.
func (s *WorkloadIdentityAPIService) FetchX509Bundles(
	_ *workloadpb.X509BundlesRequest,
	srv workloadpb.SpiffeWorkloadAPI_FetchX509BundlesServer,
) error {
	ctx := srv.Context()
	s.log.InfoContext(ctx, "FetchX509Bundles stream opened by workload")
	defer s.log.InfoContext(ctx, "FetchX509Bundles stream has closed")

	for {
		bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
		if err != nil {
			return trace.Wrap(err, "fetching trust bundle set from cache")
		}
		crlSet, err := s.crlCache.GetCRLSet(ctx)
		if err != nil {
			return trace.Wrap(err, "fetching CRL set from cache")
		}

		s.log.InfoContext(ctx, "Sending X.509 trust bundles to workload")
		resp := &workloadpb.X509BundlesResponse{
			Bundles: bundleSet.EncodedX509Bundles(true),
		}
		if len(crlSet.LocalCRL) > 0 {
			// TODO: Copy?
			resp.Crl = [][]byte{crlSet.LocalCRL}
		}
		err = srv.Send(resp)
		if err != nil {
			return trace.Wrap(err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-bundleSet.Stale():
		case <-crlSet.Stale():
		}
	}
}

// fetchX509SVIDs fetches the X.509 SVIDs for the bot's configured SVIDs and
// returns them in the SPIFFE Workload API format.
func (s *WorkloadIdentityAPIService) fetchX509SVIDs(
	ctx context.Context,
	log *slog.Logger,
	localBundle *spiffebundle.Bundle,
	attest *workloadidentityv1pb.WorkloadAttrs,
) ([]*workloadpb.X509SVID, error) {
	ctx, span := tracer.Start(ctx, "WorkloadIdentityAPIService/fetchX509SVIDs")
	defer span.End()

	creds, privateKey, err := workloadidentity.IssueX509WorkloadIdentity(
		ctx,
		log,
		s.client,
		s.cfg.Selector,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
		attest,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert the private key to PKCS#8 format as per SPIFFE spec.
	pkcs8PrivateKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	marshaledBundle := workloadidentity.MarshalX509Bundle(localBundle.X509Bundle())

	// Convert responses from the Teleport API to the SPIFFE Workload API
	// format.
	svids := make([]*workloadpb.X509SVID, len(creds))
	for i, cred := range creds {
		svids[i] = &workloadpb.X509SVID{
			// Required. The SPIFFE ID of the SVID in this entry
			SpiffeId: cred.SpiffeId,
			// Required. ASN.1 DER encoded certificate chain. MAY include
			// intermediates, the leaf certificate (or SVID itself) MUST come first.
			X509Svid: cred.GetX509Svid().GetCert(),
			// Required. ASN.1 DER encoded PKCS#8 private key. MUST be unencrypted.
			X509SvidKey: pkcs8PrivateKey,
			// Required. ASN.1 DER encoded X.509 bundle for the trust domain.
			Bundle: marshaledBundle,
			Hint:   cred.Hint,
		}
		// Log a message which correlates with the audit log entry and can
		// provide additional metadata about the client.
		log.InfoContext(ctx,
			"Issued Workload Identity Credential",
			slog.Group("credential",
				"type", "x509-svid",
				"spiffe_id", cred.SpiffeId,
				"serial_number", cred.GetX509Svid().GetSerialNumber(),
				"hint", cred.Hint,
				"expires_at", cred.ExpiresAt,
				"ttl", cred.Ttl,
				"workload_identity_name", cred.WorkloadIdentityName,
				"workload_identity_revision", cred.WorkloadIdentityRevision,
			),
		)
	}

	return svids, nil
}

// FetchJWTSVID implements the SPIFFE Workload API FetchJWTSVID method.
// See The SPIFFE Workload API (6.2.1).
func (s *WorkloadIdentityAPIService) FetchJWTSVID(
	ctx context.Context,
	req *workloadpb.JWTSVIDRequest,
) (*workloadpb.JWTSVIDResponse, error) {
	log, attr, err := s.authenticateClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "authenticating client")
	}

	log.InfoContext(ctx, "FetchJWTSVID request received from workload")
	defer log.InfoContext(ctx, "FetchJWTSVID request handled")
	if req.SpiffeId == "" {
		log = log.With("requested_spiffe_id", req.SpiffeId)
	}

	// The SPIFFE Workload API (6.2.1):
	// > The JWTSVIDRequest request message contains a mandatory audience field,
	// > which MUST contain the value to embed in the audience claim of the
	// > returned JWT-SVIDs.
	if len(req.Audience) == 0 {
		return nil, trace.BadParameter("audience: must have at least one value")
	}

	creds, err := workloadidentity.IssueJWTWorkloadIdentity(
		ctx,
		log,
		s.client,
		s.cfg.Selector,
		req.Audience,
		cmp.Or(s.cfg.CredentialLifetime, s.botCfg.CredentialLifetime).TTL,
		attr,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The SPIFFE Workload API (6.2.1):
	// > If the client is not authorized for any identities, or not authorized
	// > for the specific identity requested via the spiffe_id field, then the
	// > server SHOULD respond with the "PermissionDenied" gRPC status code.
	if len(creds) == 0 {
		log.ErrorContext(ctx, "Workload did not pass attestation for any SVIDs")
		return nil, status.Error(
			codes.PermissionDenied,
			"workload did not pass attestation for any SVIDs",
		)
	}

	svids := []*workloadpb.JWTSVID{}
	for _, cred := range creds {
		svids = append(svids, &workloadpb.JWTSVID{
			SpiffeId: cred.SpiffeId,
			Svid:     cred.GetJwtSvid().GetJwt(),
			Hint:     cred.Hint,
		})
		log.InfoContext(ctx,
			"Issued Workload Identity Credential",
			slog.Group("credential",
				"type", "jwt-svid",
				"spiffe_id", cred.SpiffeId,
				"jti", cred.GetJwtSvid().GetJti(),
				"hint", cred.Hint,
				"expires_at", cred.ExpiresAt,
				"ttl", cred.Ttl,
				"audiences", req.Audience,
			),
		)
	}

	// The SPIFFE Workload API (6.2.1):
	// > The spiffe_id field is optional, and is used to request a JWT-SVID for
	// > a specific SPIFFE ID. If unspecified, the server MUST return JWT-SVIDs
	// > for all identities authorized for the client.
	// TODO(noah): We should optimize here by making the Teleport
	// WorkloadIdentityIssuance API aware of the requested SPIFFE ID. Theres's
	// no point signing a credential to just bin it here...
	if req.SpiffeId != "" {
		requestedSPIFFEID, err := spiffeid.FromString(req.SpiffeId)
		if err != nil {
			return nil, trace.Wrap(err, "parsing requested SPIFFE ID")
		}
		if requestedSPIFFEID.TrustDomain() != s.localTrustDomain {
			return nil, trace.BadParameter("requested SPIFFE ID is not in the local trust domain")
		}

		// Search through available SVIDs to find the one that matches the
		// requested SPIFFE ID.
		found := false
		for _, svid := range svids {
			if svid.SpiffeId == req.SpiffeId {
				found = true
				svids = []*workloadpb.JWTSVID{svid}
				break
			}
		}
		if !found {
			log.ErrorContext(ctx, "Workload is not authorized for the specifically requested SPIFFE ID", "requested_spiffe_id", req.SpiffeId)
			return nil, status.Error(
				codes.PermissionDenied,
				"workload is not authorized for requested SPIFFE ID",
			)
		}
	}

	return &workloadpb.JWTSVIDResponse{
		Svids: svids,
	}, nil
}

// FetchJWTBundles implements the SPIFFE Workload API FetchJWTBundles method.
// See The SPIFFE Workload API (6.2.2).
func (s *WorkloadIdentityAPIService) FetchJWTBundles(
	_ *workloadpb.JWTBundlesRequest,
	srv workloadpb.SpiffeWorkloadAPI_FetchJWTBundlesServer,
) error {
	ctx := srv.Context()
	s.log.InfoContext(ctx, "FetchJWTBundles stream started by workload")
	defer s.log.InfoContext(ctx, "FetchJWTBundles stream ended")

	for {
		bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		s.log.InfoContext(ctx, "Sending JWT trust bundles to workload")

		// The SPIFFE Workload API (6.2.2):
		// > The returned bundles are encoded as a standard JWK Set as defined
		// > by RFC 7517 containing the JWT-SVID signing keys for the trust
		// > domain. These keys may only represent a subset of the keys present
		// > in the SPIFFE trust bundle for the trust domain. The server MUST
		// > NOT include keys with other uses in the returned JWT bundles.
		bundles, err := bundleSet.MarshaledJWKSBundles(true)
		if err != nil {
			return trace.Wrap(err, "marshaling bundles as JWKS")
		}
		err = srv.Send(&workloadpb.JWTBundlesResponse{
			Bundles: bundles,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-bundleSet.Stale():
		}
	}
}

// ValidateJWTSVID implements the SPIFFE Workload API ValidateJWTSVID method.
// See The SPIFFE Workload API (6.2.3).
func (s *WorkloadIdentityAPIService) ValidateJWTSVID(
	ctx context.Context,
	req *workloadpb.ValidateJWTSVIDRequest,
) (*workloadpb.ValidateJWTSVIDResponse, error) {
	s.log.InfoContext(ctx, "ValidateJWTSVID request received from workload")
	defer s.log.InfoContext(ctx, "ValidateJWTSVID request handled")

	// The SPIFFE Workload API (6.2.3):
	// > All fields in the ValidateJWTSVIDRequest and ValidateJWTSVIDResponse
	// > message are mandatory.
	switch {
	case req.Audience == "":
		return nil, trace.BadParameter("audience: must be set")
	case req.Svid == "":
		return nil, trace.BadParameter("svid: must be set")
	}

	bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	svid, err := jwtsvid.ParseAndValidate(
		req.Svid, bundleSet, []string{req.Audience},
	)
	if err != nil {
		return nil, trace.Wrap(err, "validating JWT SVID")
	}

	claims, err := structpb.NewStruct(svid.Claims)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling claims")
	}

	return &workloadpb.ValidateJWTSVIDResponse{
		SpiffeId: svid.ID.String(),
		Claims:   claims,
	}, nil
}

// String returns a human-readable string that can uniquely identify the
// service.
func (s *WorkloadIdentityAPIService) String() string {
	return fmt.Sprintf("%s:%s", config.WorkloadIdentityAPIServiceType, s.cfg.Listen)
}
