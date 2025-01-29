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
	"cmp"
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/gravitational/teleport"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
	"github.com/gravitational/teleport/lib/uds"
	utilsuds "github.com/gravitational/teleport/lib/utils/uds"
)

// SPIFFEWorkloadAPIService implements a gRPC server that fulfills the SPIFFE
// Workload API specification. It provides X509 SVIDs and trust bundles to
// workloads that connect over the configured listener.
//
// Sources:
// - https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_Endpoint.md
// - https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_API.md
// - https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md
// - https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md
type SPIFFEWorkloadAPIService struct {
	workloadpb.UnimplementedSpiffeWorkloadAPIServer

	svcIdentity      *config.UnstableClientCredentialOutput
	botCfg           *config.BotConfig
	cfg              *config.SPIFFEWorkloadAPIService
	log              *slog.Logger
	resolver         reversetunnelclient.Resolver
	trustBundleCache *workloadidentity.TrustBundleCache

	// client holds the impersonated client for the service
	client           *authclient.Client
	attestor         *workloadattest.Attestor
	localTrustDomain spiffeid.TrustDomain
}

// setup initializes the service, performing tasks such as determining the
// trust domain, fetching the initial trust bundle and creating an impersonated
// client.
func (s *SPIFFEWorkloadAPIService) setup(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "SPIFFEWorkloadAPIService/setup")
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

func createListener(ctx context.Context, log *slog.Logger, addr string) (net.Listener, error) {
	parsed, err := url.Parse(addr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing %q", addr)
	}

	switch parsed.Scheme {
	// If no scheme is provided, default to TCP.
	case "tcp", "":
		return net.Listen("tcp", parsed.Host)
	case "unix":
		absPath, err := filepath.Abs(parsed.Path)
		if err != nil {
			return nil, trace.Wrap(err, "resolving absolute path for %q", parsed.Path)
		}

		// Remove the file if it already exists. This is necessary to handle
		// unclean exits.
		if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
			log.WarnContext(ctx, "Failed to remove existing socket file", "error", err)
		}

		l, err := utilsuds.ListenUnix(ctx, "unix", absPath)
		if err != nil {
			return nil, trace.Wrap(err, "creating unix socket", absPath)
		}

		// On Unix systems, you must have read and write permissions for the
		// socket to connect to it. The execute permission on the directories
		// containing the socket must also be granted. This is different to when
		// we write output artifacts which only require the consumer to have
		// read access.
		//
		// We set the socket perm to 777. Instead of controlling access via
		// the socket file directly, users will either:
		// - Configure Unix Workload Attestation to restrict access to specific
		//   PID/UID/GID combinations.
		// - Configure the filesystem permissions of the directory containing
		//   the socket.
		if err := os.Chmod(absPath, os.ModePerm); err != nil {
			return nil, trace.Wrap(err, "setting permissions on unix socket", absPath)
		}

		return l, nil
	default:
		return nil, trace.BadParameter("unsupported scheme %q", parsed.Scheme)
	}
}

func (s *SPIFFEWorkloadAPIService) Run(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "SPIFFEWorkloadAPIService/Run")
	defer span.End()

	s.log.DebugContext(ctx, "Starting pre-run initialization")
	if err := s.setup(ctx); err != nil {
		return trace.Wrap(err)
	}
	defer s.client.Close()
	s.log.DebugContext(ctx, "Completed pre-run initialization")

	srvMetrics := metrics.CreateGRPCServerMetrics(
		true, prometheus.Labels{
			teleport.TagServer: "tbot-spiffe-workload-api",
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
				return s.fetchX509SVIDs(
					ctx,
					log,
					localBundle,
					filterSVIDRequests(ctx, log, s.cfg.SVIDs, attrs),
				)
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

// serialString returns a human-readable colon-separated string of the serial
// number in hex.
func serialString(serial *big.Int) string {
	hex := serial.Text(16)
	if len(hex)%2 == 1 {
		hex = "0" + hex
	}

	out := strings.Builder{}
	for i := 0; i < len(hex); i += 2 {
		if i != 0 {
			out.WriteString(":")
		}
		out.WriteString(hex[i : i+2])
	}
	return out.String()
}

// fetchX509SVIDs fetches the X.509 SVIDs for the bot's configured SVIDs and
// returns them in the SPIFFE Workload API format.
func (s *SPIFFEWorkloadAPIService) fetchX509SVIDs(
	ctx context.Context,
	log *slog.Logger,
	localBundle *spiffebundle.Bundle,
	svidRequests []config.SVIDRequest,
) ([]*workloadpb.X509SVID, error) {
	ctx, span := tracer.Start(ctx, "SPIFFEWorkloadAPIService/fetchX509SVIDs")
	defer span.End()

	// TODO(noah): We should probably take inspiration from SPIRE agent's
	// behavior of pre-fetching the SVIDs rather than doing this for
	// every request.
	res, privateKey, err := generateSVID(
		ctx,
		s.client,
		svidRequests,
		// For TTL, we use the one globally configured.
		s.botCfg.CertificateTTL,
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
	svids := make([]*workloadpb.X509SVID, len(res.Svids))
	for i, svidRes := range res.Svids {
		svids[i] = &workloadpb.X509SVID{
			// Required. The SPIFFE ID of the SVID in this entry
			SpiffeId: svidRes.SpiffeId,
			// Required. ASN.1 DER encoded certificate chain. MAY include
			// intermediates, the leaf certificate (or SVID itself) MUST come first.
			X509Svid: svidRes.Certificate,
			// Required. ASN.1 DER encoded PKCS#8 private key. MUST be unencrypted.
			X509SvidKey: pkcs8PrivateKey,
			// Required. ASN.1 DER encoded X.509 bundle for the trust domain.
			Bundle: marshaledBundle,
			Hint:   svidRes.Hint,
		}
		cert, err := x509.ParseCertificate(svidRes.Certificate)
		if err != nil {
			return nil, trace.Wrap(err, "parsing issued svid received from server")
		}
		// Log a message which correlates with the audit log entry and can
		// provide additional metadata about the client.
		log.InfoContext(ctx,
			"Issued SVID for workload",
			slog.Group("svid",
				"type", "x509",
				"spiffe_id", svidRes.SpiffeId,
				"serial_number", serialString(cert.SerialNumber),
				"hint", svidRes.Hint,
				"not_after", cert.NotAfter,
				"not_before", cert.NotBefore,
				"dns_sans", cert.DNSNames,
				"ip_sans", cert.IPAddresses,
			),
		)
	}

	return svids, nil
}

// filterSVIDRequests filters the SVID requests based on the workload
// attestation.
//
// TODO(noah): In a future PR, we need to totally refactor this to a more
// flexible rules engine otherwise this is going to get absurdly large as
// we add more types. Ideally, something that would be compatible with a
// predicate language would be great.
func filterSVIDRequests(
	ctx context.Context,
	log *slog.Logger,
	svidRequests []config.SVIDRequestWithRules,
	att *workloadidentityv1pb.WorkloadAttrs,
) []config.SVIDRequest {
	var filtered []config.SVIDRequest
	for _, req := range svidRequests {
		log := log.With("svid", req.SVIDRequest)
		// If no rules are configured, default to allow.
		if len(req.Rules) == 0 {
			log.DebugContext(
				ctx,
				"No rules configured for SVID. SVID will be issued",
			)
			filtered = append(filtered, req.SVIDRequest)
			continue
		}

		// Otherwise, evaluate all the rules, looking for one matching rule.
		match := false
		for _, rule := range req.Rules {
			log := log.With("rule", rule)
			logMismatch := func(field string, want any, got any) {
				log.DebugContext(
					ctx,
					"Rule did not match workload attestation",
					"field", field,
					"want", want,
					"got", got,
				)
			}
			logNotAttested := func(requiredAttestor string) {
				log.DebugContext(
					ctx,
					"Workload did not complete attestation required for this rule",
					"required_attestor", requiredAttestor,
				)
			}
			log.DebugContext(
				ctx,
				"Evaluating rule against workload attestation",
			)
			if rule.Unix.UID != nil {
				if !att.GetUnix().GetAttested() {
					logNotAttested("unix")
					continue
				}
				if *rule.Unix.UID != int(att.GetUnix().GetUid()) {
					logMismatch("unix.uid", *rule.Unix.UID, att.GetUnix().GetUid())
					continue
				}
				// Rule field matched!
			}
			if rule.Unix.PID != nil {
				if !att.GetUnix().GetAttested() {
					logNotAttested("unix")
					continue
				}
				if *rule.Unix.PID != int(att.GetUnix().GetPid()) {
					logMismatch("unix.pid", *rule.Unix.PID, att.GetUnix().GetPid())
					continue
				}
				// Rule field matched!
			}
			if rule.Unix.GID != nil {
				if !att.GetUnix().GetAttested() {
					logNotAttested("unix")
					continue
				}
				if *rule.Unix.GID != int(att.GetUnix().GetGid()) {
					logMismatch("unix.gid", *rule.Unix.GID, att.GetUnix().GetGid())
					continue
				}
				// Rule field matched!
			}
			if rule.Kubernetes.Namespace != "" {
				if !att.GetKubernetes().GetAttested() {
					logNotAttested("kubernetes")
					continue
				}
				if rule.Kubernetes.Namespace != att.GetKubernetes().GetNamespace() {
					logMismatch("kubernetes.namespace", rule.Kubernetes.Namespace, att.GetKubernetes().GetNamespace())
					continue
				}
				// Rule field matched!
			}
			if rule.Kubernetes.PodName != "" {
				if !att.GetKubernetes().GetAttested() {
					logNotAttested("kubernetes")
					continue
				}
				if rule.Kubernetes.PodName != att.GetKubernetes().GetPodName() {
					logMismatch("kubernetes.pod_name", rule.Kubernetes.PodName, att.GetKubernetes().GetPodName())
					continue
				}
				// Rule field matched!
			}
			if rule.Kubernetes.ServiceAccount != "" {
				if !att.GetKubernetes().GetAttested() {
					logNotAttested("kubernetes")
					continue
				}
				if rule.Kubernetes.ServiceAccount != att.GetKubernetes().GetServiceAccount() {
					logMismatch("kubernetes.service_account", rule.Kubernetes.ServiceAccount, att.GetKubernetes().GetServiceAccount())
					continue
				}
				// Rule field matched!
			}

			log.DebugContext(
				ctx,
				"Rule matched workload attestation. SVID will be issued",
			)
			match = true
			filtered = append(filtered, req.SVIDRequest)
			break
		}
		if !match {
			log.DebugContext(
				ctx,
				"No rules matched workload attestation. SVID will not be issued",
			)
		}
	}
	return filtered
}

func (s *SPIFFEWorkloadAPIService) authenticateClient(
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
func (s *SPIFFEWorkloadAPIService) FetchX509SVID(
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

	// Before we issue the SVIDs to the workload, we need to complete workload
	// attestation and determine which SVIDs to issue.
	svidReqs := filterSVIDRequests(ctx, log, s.cfg.SVIDs, creds)

	// The SPIFFE Workload API (5.2.1):
	//
	//   If the client is not entitled to receive any X509-SVIDs, then the
	//   server SHOULD respond with the "PermissionDenied" gRPC status code (see
	//   the Error Codes section in the SPIFFE Workload Endpoint specification
	//   for more information). Under such a case, the client MAY attempt to
	//   reconnect with another call to the FetchX509SVID RPC after a backoff.
	if len(svidReqs) == 0 {
		log.ErrorContext(ctx, "Workload did not pass attestation for any SVIDs")
		return status.Error(
			codes.PermissionDenied,
			"workload did not pass attestation for any SVIDs",
		)
	}

	bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	var svids []*workloadpb.X509SVID
	for {
		log.InfoContext(ctx, "Starting to issue X509 SVIDs to workload")

		// Fetch SVIDs if necessary.
		if svids == nil {
			svids, err = s.fetchX509SVIDs(ctx, log, bundleSet.Local, svidReqs)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		err = srv.Send(&workloadpb.X509SVIDResponse{
			Svids:            svids,
			FederatedBundles: bundleSet.EncodedX509Bundles(false),
		})
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
		case <-time.After(s.botCfg.RenewalInterval):
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
func (s *SPIFFEWorkloadAPIService) FetchX509Bundles(
	_ *workloadpb.X509BundlesRequest,
	srv workloadpb.SpiffeWorkloadAPI_FetchX509BundlesServer,
) error {
	ctx := srv.Context()
	s.log.InfoContext(ctx, "FetchX509Bundles stream opened by workload")
	defer s.log.InfoContext(ctx, "FetchX509Bundles stream has closed")

	for {
		bundleSet, err := s.trustBundleCache.GetBundleSet(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		s.log.InfoContext(ctx, "Sending X.509 trust bundles to workload")
		err = srv.Send(&workloadpb.X509BundlesResponse{
			Bundles: bundleSet.EncodedX509Bundles(true),
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

const defaultJWTSVIDTTL = time.Minute * 5

// FetchJWTSVID implements the SPIFFE Workload API FetchJWTSVID method.
// See The SPIFFE Workload API (6.2.1).
func (s *SPIFFEWorkloadAPIService) FetchJWTSVID(
	ctx context.Context,
	req *workloadpb.JWTSVIDRequest,
) (*workloadpb.JWTSVIDResponse, error) {
	log, creds, err := s.authenticateClient(ctx)
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

	svidReqs := filterSVIDRequests(ctx, log, s.cfg.SVIDs, creds)
	// The SPIFFE Workload API (6.2.1):
	// > If the client is not authorized for any identities, or not authorized
	// > for the specific identity requested via the spiffe_id field, then the
	// > server SHOULD respond with the "PermissionDenied" gRPC status code.
	if len(svidReqs) == 0 {
		log.ErrorContext(ctx, "Workload did not pass attestation for any SVIDs")
		return nil, status.Error(
			codes.PermissionDenied,
			"workload did not pass attestation for any SVIDs",
		)
	}

	// The SPIFFE Workload API (6.2.1):
	// > The spiffe_id field is optional, and is used to request a JWT-SVID for
	// > a specific SPIFFE ID. If unspecified, the server MUST return JWT-SVIDs
	// > for all identities authorized for the client.
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
		for _, svidReq := range svidReqs {
			spiffeID, err := spiffeid.FromPath(s.localTrustDomain, svidReq.Path)
			if err != nil {
				return nil, trace.Wrap(err, "parsing SPIFFE ID from path %q", svidReq.Path)
			}
			if spiffeID.String() == req.SpiffeId {
				found = true
				svidReqs = []config.SVIDRequest{svidReq}
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

	// Allow users to manually override the TTL for JWT-SVIDs produced by this
	// service.
	ttl := cmp.Or(s.cfg.JWTSVIDTTL, defaultJWTSVIDTTL)

	reqs := make([]*machineidv1pb.JWTSVIDRequest, 0, len(svidReqs))
	for _, svidReq := range svidReqs {
		reqs = append(reqs, &machineidv1pb.JWTSVIDRequest{
			SpiffeIdPath: svidReq.Path,
			Audiences:    req.Audience,
			Ttl:          durationpb.New(ttl),
			Hint:         svidReq.Hint,
		})
	}

	res, err := s.client.WorkloadIdentityServiceClient().SignJWTSVIDs(ctx, &machineidv1pb.SignJWTSVIDsRequest{
		Svids: reqs,
	})
	if err != nil {
		return nil, trace.Wrap(err, "requesting signed JWT-SVIDs from Teleport")
	}

	out := &workloadpb.JWTSVIDResponse{}
	for _, resSVID := range res.Svids {
		out.Svids = append(out.Svids, &workloadpb.JWTSVID{
			SpiffeId: resSVID.SpiffeId,
			Svid:     resSVID.Jwt,
			Hint:     resSVID.Hint,
		})
		log.InfoContext(ctx,
			"Issued SVID for workload",
			slog.Group("svid",
				"type", "jwt",
				"spiffe_id", resSVID.SpiffeId,
				"jti", resSVID.Jti,
				"hint", resSVID.Hint,
				"audiences", resSVID.Audiences,
			),
		)
	}

	return out, nil
}

// FetchJWTBundles implements the SPIFFE Workload API FetchJWTBundles method.
// See The SPIFFE Workload API (6.2.2).
func (s *SPIFFEWorkloadAPIService) FetchJWTBundles(
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
func (s *SPIFFEWorkloadAPIService) ValidateJWTSVID(
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
func (s *SPIFFEWorkloadAPIService) String() string {
	return fmt.Sprintf("%s:%s", config.SPIFFEWorkloadAPIServiceType, s.cfg.Listen)
}
