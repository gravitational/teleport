/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/moby/term"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/socks"
)

// ProxyClient implements ssh client to a teleport proxy
// It can provide list of nodes or connect to nodes
type ProxyClient struct {
	teleportClient  *TeleportClient
	Client          *tracessh.Client
	Tracer          oteltrace.Tracer
	hostLogin       string
	proxyAddress    string
	proxyPrincipal  string
	hostKeyCallback ssh.HostKeyCallback
	authMethods     []ssh.AuthMethod
	siteName        string
	clientAddr      string

	// currentCluster is a client for the local teleport auth server that
	// the proxy is connected to. This should be reused for the duration
	// of the ProxyClient to ensure the auth server is only dialed once.
	currentCluster authclient.ClientI
}

// NodeClient implements ssh client to a ssh node (teleport or any regular ssh node)
// NodeClient can run shell and commands or upload and download files.
type NodeClient struct {
	Namespace   string
	Tracer      oteltrace.Tracer
	Client      *tracessh.Client
	TC          *TeleportClient
	OnMFA       func()
	FIPSEnabled bool

	mu      sync.Mutex
	closers []io.Closer

	// ProxyPublicAddr is the web proxy public addr, as opposed to the local proxy
	// addr set in TC.WebProxyAddr. This is needed to report the correct address
	// to SSH_TELEPORT_WEBPROXY_ADDR used by some features like "teleport status".
	ProxyPublicAddr string

	// hostname is the node's hostname, for more user-friendly logging.
	hostname string

	// sshLogDir is the directory to log the output of multiple SSH commands to.
	// If not set, no logs will be created.
	sshLogDir string
}

// AddCloser adds an [io.Closer] that will be closed when the
// client is closed.
func (c *NodeClient) AddCloser(closer io.Closer) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closers = append(c.closers, closer)
}

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
}

// AddCancel adds a [context.CancelFunc] that will be canceled when the
// client is closed.
func (c *NodeClient) AddCancel(cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closers = append(c.closers, closerFunc(func() error {
		cancel()
		return nil
	}))
}

// ClusterName returns the name of the cluster the proxy is a member of.
func (proxy *ProxyClient) ClusterName() string {
	return proxy.siteName
}

// GetSites returns list of the "sites" (AKA teleport clusters) connected to the proxy
// Each site is returned as an instance of its auth server
func (proxy *ProxyClient) GetSites(ctx context.Context) ([]types.Site, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/GetSites",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", proxy.siteName),
		),
	)
	defer span.End()

	proxySession, err := proxy.Client.NewSession(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxySession.Close()
	stdout := &bytes.Buffer{}
	reader, err := proxySession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	done := make(chan struct{})
	go func() {
		if _, err := io.Copy(stdout, reader); err != nil {
			log.Warningf("Error reading STDOUT from proxy: %v", err)
		}
		close(done)
	}()
	// this function is async because,
	// the function call StdoutPipe() could fail if proxy rejected
	// the session request, and then RequestSubsystem call could hang
	// forever
	go func() {
		if err := proxySession.RequestSubsystem(ctx, "proxysites"); err != nil {
			log.Warningf("Failed to request subsystem: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(apidefaults.DefaultIOTimeout):
		return nil, trace.ConnectionProblem(nil, "timeout")
	}
	log.Debugf("Found clusters: %v", stdout.String())
	var sites []types.Site
	if err := json.Unmarshal(stdout.Bytes(), &sites); err != nil {
		return nil, trace.Wrap(err)
	}
	return sites, nil
}

// GetLeafClusters returns the leaf/remote clusters.
func (proxy *ProxyClient) GetLeafClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/GetLeafClusters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", proxy.siteName),
		),
	)
	defer span.End()

	clt, err := proxy.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	remoteClusters, err := clt.GetRemoteClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return remoteClusters, nil
}

// ReissueParams encodes optional parameters for
// user certificate reissue.
type ReissueParams struct {
	RouteToCluster    string
	NodeName          string
	KubernetesCluster string
	AccessRequests    []string
	// See [proto.UserCertsRequest.DropAccessRequests].
	DropAccessRequests    []string
	RouteToDatabase       proto.RouteToDatabase
	RouteToApp            proto.RouteToApp
	RouteToWindowsDesktop proto.RouteToWindowsDesktop

	// ExistingCreds is a gross hack for lib/web/terminal.go to pass in
	// existing user credentials. The TeleportClient in lib/web/terminal.go
	// doesn't have a real LocalKeystore and keeps all certs in memory.
	// Normally, existing credentials are loaded from
	// TeleportClient.localAgent.
	//
	// TODO(awly): refactor lib/web to use a Keystore implementation that
	// mimics LocalKeystore and remove this.
	ExistingCreds *Key

	// MFACheck is optional parameter passed if MFA check was already done.
	// It can be nil.
	MFACheck *proto.IsMFARequiredResponse
	// AuthClient is the client used for the MFACheck that can be reused
	AuthClient authclient.ClientI
	// RequesterName identifies who is sending the cert reissue request.
	RequesterName proto.UserCertsRequest_Requester
}

func (p ReissueParams) usage() proto.UserCertsRequest_CertUsage {
	switch {
	case p.NodeName != "":
		// SSH means a request for an SSH certificate for access to a specific
		// SSH node, as specified by NodeName.
		return proto.UserCertsRequest_SSH
	case p.KubernetesCluster != "":
		// Kubernetes means a request for a TLS certificate for access to a
		// specific Kubernetes cluster, as specified by KubernetesCluster.
		return proto.UserCertsRequest_Kubernetes
	case p.RouteToDatabase.ServiceName != "":
		// Database means a request for a TLS certificate for access to a
		// specific database, as specified by RouteToDatabase.
		return proto.UserCertsRequest_Database
	case p.RouteToApp.Name != "":
		// App means a request for a TLS certificate for access to a specific
		// web app, as specified by RouteToApp.
		return proto.UserCertsRequest_App
	case p.RouteToWindowsDesktop.WindowsDesktop != "":
		return proto.UserCertsRequest_WindowsDesktop
	default:
		// All means a request for both SSH and TLS certificates for the
		// overall user session. These certificates are not specific to any SSH
		// node, Kubernetes cluster, database or web app.
		return proto.UserCertsRequest_All
	}
}

func (p ReissueParams) isMFARequiredRequest(sshLogin string) *proto.IsMFARequiredRequest {
	req := new(proto.IsMFARequiredRequest)
	switch {
	case p.NodeName != "":
		req.Target = &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{Node: p.NodeName, Login: sshLogin}}
	case p.KubernetesCluster != "":
		req.Target = &proto.IsMFARequiredRequest_KubernetesCluster{KubernetesCluster: p.KubernetesCluster}
	case p.RouteToDatabase.ServiceName != "":
		req.Target = &proto.IsMFARequiredRequest_Database{Database: &p.RouteToDatabase}
	case p.RouteToWindowsDesktop.WindowsDesktop != "":
		req.Target = &proto.IsMFARequiredRequest_WindowsDesktop{WindowsDesktop: &p.RouteToWindowsDesktop}
	}
	return req
}

// CertCachePolicy describes what should happen to the certificate cache when a
// user certificate is re-issued
type CertCachePolicy int

const (
	// CertCacheDrop indicates that all user certificates should be dropped as
	// part of the re-issue process. This can be necessary if the roles
	// assigned to the user are expected to change as a part of the re-issue.
	CertCacheDrop CertCachePolicy = 0

	// CertCacheKeep indicates that all user certificates (except those
	// explicitly updated by the re-issue) should be preserved across the
	// re-issue process.
	CertCacheKeep CertCachePolicy = 1
)

// ReissueUserCerts generates certificates for the user
// that have a metadata instructing server to route the requests to the cluster
func (proxy *ProxyClient) ReissueUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) error {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/ReissueUserCerts",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", proxy.siteName),
		),
	)
	defer span.End()

	key, err := proxy.reissueUserCerts(ctx, cachePolicy, params)
	if err != nil {
		return trace.Wrap(err)
	}

	if cachePolicy == CertCacheDrop {
		proxy.localAgent().DeleteUserCerts("", WithAllCerts...)
	}

	// save the cert to the local storage (~/.tsh usually):
	err = proxy.localAgent().AddKey(key)
	return trace.Wrap(err)
}

func (proxy *ProxyClient) reissueUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) (*Key, error) {
	if params.RouteToCluster == "" {
		params.RouteToCluster = proxy.siteName
	}
	key := params.ExistingCreds
	if key == nil {
		var err error

		// Don't load the certs if we're going to drop all of them all as part
		// of the re-issue. If we load all of the old certs now we won't be able
		// to differentiate between legacy certificates (that need to be
		// deleted) and newly re-issued certs (that we definitely do *not* want
		// to delete) when it comes time to drop them from the local agent.
		certOptions := []CertOption{}
		if cachePolicy == CertCacheKeep {
			certOptions = WithAllCerts
		}

		key, err = proxy.localAgent().GetKey(params.RouteToCluster, certOptions...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	req, err := proxy.prepareUserCertsRequest(params, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := proxy.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()
	certs, err := clt.GenerateUserCerts(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.ClusterName = params.RouteToCluster

	// Only update the parts of key that match the usage. See the docs on
	// proto.UserCertsRequest_CertUsage for which certificates match which
	// usage.
	//
	// This prevents us from overwriting the top-level key.TLSCert with
	// usage-restricted certificates.
	switch params.usage() {
	case proto.UserCertsRequest_All:
		key.Cert = certs.SSH
		key.TLSCert = certs.TLS
	case proto.UserCertsRequest_SSH:
		key.Cert = certs.SSH
	case proto.UserCertsRequest_App:
		key.AppTLSCerts[params.RouteToApp.Name] = certs.TLS
	case proto.UserCertsRequest_Database:
		dbCert, err := makeDatabaseClientPEM(params.RouteToDatabase.Protocol, certs.TLS, key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		key.DBTLSCerts[params.RouteToDatabase.ServiceName] = dbCert
	case proto.UserCertsRequest_Kubernetes:
		key.KubeTLSCerts[params.KubernetesCluster] = certs.TLS
	case proto.UserCertsRequest_WindowsDesktop:
		key.WindowsDesktopCerts[params.RouteToWindowsDesktop.WindowsDesktop] = certs.TLS
	}
	return key, nil
}

// makeDatabaseClientPEM returns appropriate client PEM file contents for the
// specified database type. Some databases only need certificate in the PEM
// file, others both certificate and key.
func makeDatabaseClientPEM(proto string, cert []byte, pk *Key) ([]byte, error) {
	// MongoDB expects certificate and key pair in the same pem file.
	if proto == defaults.ProtocolMongoDB {
		rsaKeyPEM, err := pk.PrivateKey.RSAPrivateKeyPEM()
		if err == nil {
			return append(cert, rsaKeyPEM...), nil
		} else if !trace.IsBadParameter(err) {
			return nil, trace.Wrap(err)
		}
		log.WithError(err).Warn("MongoDB integration is not supported when logging in with a non-rsa private key.")
	}
	return cert, nil
}

// PromptMFAChallengeHandler is a handler for MFA challenges.
//
// The challenge c from proxyAddr should be presented to the user, asking to
// use one of their registered MFA devices. User's response should be returned,
// or an error if anything goes wrong.
type PromptMFAChallengeHandler func(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error)

// issueUserCertsOpts contains extra options for issuing user certs.
type issueUserCertsOpts struct {
	mfaRequired *bool
}

// IssueUserCertsOpt is an option func for issuing user certs.
type IssueUserCertsOpt func(*issueUserCertsOpts)

// WithMFARequired is an IssueUserCertsOpt that sets the MFA required check
// result in provided bool ptr.
func WithMFARequired(mfaRequired *bool) IssueUserCertsOpt {
	return func(opt *issueUserCertsOpts) {
		opt.mfaRequired = mfaRequired
	}
}

// IssueUserCertsWithMFA generates a single-use certificate for the user.
func (proxy *ProxyClient) IssueUserCertsWithMFA(ctx context.Context, params ReissueParams, mfaPrompt mfa.Prompt, applyOpts ...IssueUserCertsOpt) (*Key, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/IssueUserCertsWithMFA",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", proxy.siteName),
		),
	)
	defer span.End()

	issueOpts := issueUserCertsOpts{}
	for _, applyOpt := range applyOpts {
		applyOpt(&issueOpts)
	}

	if params.RouteToCluster == "" {
		params.RouteToCluster = proxy.siteName
	}
	key := params.ExistingCreds
	if key == nil {
		var err error
		key, err = proxy.localAgent().GetKey(params.RouteToCluster, WithAllCerts...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var clt authclient.ClientI
	// requiredCheck passed from param can be nil.
	requiredCheck := params.MFACheck
	if requiredCheck == nil || requiredCheck.Required {
		// Connect to the target cluster (root or leaf) to check whether MFA is
		// required or if we know from param that it's required, connect because
		// it will be needed to do MFA check.
		if params.AuthClient != nil {
			clt = params.AuthClient
		} else {
			authClt, err := proxy.ConnectToCluster(ctx, params.RouteToCluster)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			clt = authClt
			defer clt.Close()
		}
	}

	if requiredCheck == nil {
		check, err := clt.IsMFARequired(ctx, params.isMFARequiredRequest(proxy.hostLogin))
		if err != nil {
			if trace.IsNotImplemented(err) {
				// Probably talking to an older server, use the old non-MFA endpoint.
				log.WithError(err).Debug("Auth server does not implement IsMFARequired.")
				// SSH certs can be used without reissuing.
				if params.usage() == proto.UserCertsRequest_SSH && key.Cert != nil {
					return key, nil
				}
				return proxy.reissueUserCerts(ctx, CertCacheKeep, params)
			}
			return nil, trace.Wrap(err)
		}
		requiredCheck = check
	}

	if issueOpts.mfaRequired != nil {
		*issueOpts.mfaRequired = requiredCheck.Required
	}

	if !requiredCheck.Required {
		log.Debug("MFA not required for access.")
		// MFA is not required.
		// SSH certs can be used without embedding the node name.
		if params.usage() == proto.UserCertsRequest_SSH && key.Cert != nil {
			return key, nil
		}
		// All other targets need their name embedded in the cert for routing,
		// fall back to non-MFA reissue.
		return proxy.reissueUserCerts(ctx, CertCacheKeep, params)
	}

	// Always connect to root for getting new credentials, but attempt to reuse
	// the existing client if possible.
	rootClusterName, err := key.RootClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if params.RouteToCluster != rootClusterName {
		clt.Close()
		rootClusterProxy := proxy
		if jumpHost := proxy.teleportClient.JumpHosts; jumpHost != nil {
			// In case of MFA connect to root teleport proxy instead of JumpHost to request
			// MFA certificates.
			proxy.teleportClient.JumpHosts = nil
			rootClusterProxy, err = proxy.teleportClient.ConnectToProxy(ctx)
			proxy.teleportClient.JumpHosts = jumpHost
			if err != nil {
				return nil, trace.Wrap(err)
			}
			defer rootClusterProxy.Close()
		}
		clt, err = rootClusterProxy.ConnectToCluster(ctx, rootClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer clt.Close()
	}

	certsReq, err := proxy.prepareUserCertsRequest(params, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, _, err = PerformMFACeremony(ctx, PerformMFACeremonyParams{
		CurrentAuthClient: proxy.currentCluster,
		RootAuthClient:    clt,
		MFAPrompt:         mfaPrompt,
		MFAAgainstRoot:    params.RouteToCluster == rootClusterName,
		MFARequiredReq:    nil, // No need to check if we got this far.
		ChallengeExtensions: mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
		CertsReq: certsReq,
		Key:      key,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debug("Issued single-use user certificate after an MFA check.")
	return key, nil
}

func (proxy *ProxyClient) prepareUserCertsRequest(params ReissueParams, key *Key) (*proto.UserCertsRequest, error) {
	tlsCert, err := key.TeleportTLSCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(params.AccessRequests) == 0 {
		// Get the active access requests to include in the cert.
		activeRequests, err := key.ActiveRequests()
		// key.ActiveRequests can return a NotFound error if it doesn't have an
		// SSH cert. That's OK, we just assume that there are no AccessRequests
		// in that case.
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		params.AccessRequests = activeRequests.AccessRequests
	}

	return &proto.UserCertsRequest{
		PublicKey:             key.MarshalSSHPublicKey(),
		Username:              tlsCert.Subject.CommonName,
		Expires:               tlsCert.NotAfter,
		RouteToCluster:        params.RouteToCluster,
		KubernetesCluster:     params.KubernetesCluster,
		AccessRequests:        params.AccessRequests,
		DropAccessRequests:    params.DropAccessRequests,
		RouteToDatabase:       params.RouteToDatabase,
		RouteToWindowsDesktop: params.RouteToWindowsDesktop,
		RouteToApp:            params.RouteToApp,
		NodeName:              params.NodeName,
		Usage:                 params.usage(),
		Format:                proxy.teleportClient.CertificateFormat,
		RequesterName:         params.RequesterName,
		AttestationStatement:  key.PrivateKey.GetAttestationStatement().ToProto(),
	}, nil
}

// RootClusterName returns name of the current cluster
func (proxy *ProxyClient) RootClusterName(ctx context.Context) (string, error) {
	_, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/RootClusterName",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	return proxy.teleportClient.RootClusterName(ctx)
}

// CreateAccessRequestV2 registers a new access request with the auth server.
func (proxy *ProxyClient) CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/CreateAccessRequestV2",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("request", req.GetName())),
	)
	defer span.End()

	site := proxy.CurrentCluster()

	return site.CreateAccessRequestV2(ctx, req)
}

// GetAccessRequests loads all access requests matching the supplied filter.
func (proxy *ProxyClient) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/GetAccessRequests",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("id", filter.ID),
			attribute.String("user", filter.User),
		),
	)
	defer span.End()

	site := proxy.CurrentCluster()

	reqs, err := site.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reqs, nil
}

// GetRole loads a role resource by name.
func (proxy *ProxyClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/GetRole",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("role", name),
		),
	)
	defer span.End()

	site := proxy.CurrentCluster()

	role, err := site.GetRole(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// NewWatcher sets up a new event watcher.
func (proxy *ProxyClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/NewWatcher",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("name", watch.Name),
		),
	)
	defer span.End()

	site := proxy.CurrentCluster()

	watcher, err := site.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return watcher, nil
}

func (proxy *ProxyClient) GetClusterAlerts(ctx context.Context, req types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/GetClusterAlerts",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	site := proxy.CurrentCluster()
	defer site.Close()

	alerts, err := site.GetClusterAlerts(ctx, req)
	return alerts, trace.Wrap(err)
}

// FindNodesByFiltersForCluster returns list of the nodes in a specified cluster which have filters matched.
func (proxy *ProxyClient) FindNodesByFiltersForCluster(ctx context.Context, req *proto.ListResourcesRequest, cluster string) ([]types.Server, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/FindNodesByFiltersForCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", cluster),
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	site, err := proxy.ConnectToCluster(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := client.GetAllResources[types.Server](ctx, site, req)
	return servers, trace.Wrap(err)
}

// FindAppServersByFilters returns a list of application servers in the current cluster which have filters matched.
func (proxy *ProxyClient) FindAppServersByFilters(ctx context.Context, req proto.ListResourcesRequest) ([]types.AppServer, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/FindAppServersByFilters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	servers, err := proxy.FindAppServersByFiltersForCluster(ctx, req, proxy.siteName)
	return servers, trace.Wrap(err)
}

// FindAppServersByFiltersForCluster returns a list of application servers for a given cluster which have filters matched.
func (proxy *ProxyClient) FindAppServersByFiltersForCluster(ctx context.Context, req proto.ListResourcesRequest, cluster string) ([]types.AppServer, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/FindAppServersByFiltersForCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", cluster),
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	authClient, err := proxy.ConnectToCluster(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := client.GetAllResources[types.AppServer](ctx, authClient, &req)
	return servers, trace.Wrap(err)
}

// CreateAppSession creates a new application access session.
func (proxy *ProxyClient) CreateAppSession(ctx context.Context, req types.CreateAppSessionRequest) (types.WebSession, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/CreateAppSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("username", req.Username),
			attribute.String("cluster", req.ClusterName),
		),
	)
	defer span.End()

	clusterName, err := proxy.RootClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authClient, err := proxy.ConnectToCluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer authClient.Close()

	ws, err := authClient.CreateAppSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Make sure to wait for the created app session to propagate through the cache.
	accessPoint, err := proxy.ConnectToCluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer accessPoint.Close()

	err = authclient.WaitForAppSession(ctx, ws.GetName(), ws.GetUser(), accessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ws, nil
}

// GetAppSession creates a new application access session.
func (proxy *ProxyClient) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/GetAppSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterName, err := proxy.RootClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authClient, err := proxy.ConnectToCluster(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ws, err := authClient.GetAppSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ws, nil
}

// DeleteAppSession removes the specified application access session.
func (proxy *ProxyClient) DeleteAppSession(ctx context.Context, sessionID string) error {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/DeleteAppSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("session", sessionID),
		),
	)
	defer span.End()

	authClient, err := proxy.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = authClient.DeleteAppSession(ctx, types.DeleteAppSessionRequest{SessionID: sessionID})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteUserAppSessions removes user's all application web sessions.
func (proxy *ProxyClient) DeleteUserAppSessions(ctx context.Context, req *proto.DeleteUserAppSessionsRequest) error {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/DeleteUserAppSessions",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("username", req.Username),
		),
	)
	defer span.End()

	authClient, err := proxy.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = authClient.DeleteUserAppSessions(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// FindDatabaseServersByFilters returns registered database proxy servers that match the provided filter.
func (proxy *ProxyClient) FindDatabaseServersByFilters(ctx context.Context, req proto.ListResourcesRequest) ([]types.DatabaseServer, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/FindDatabaseServersByFilters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	servers, err := proxy.FindDatabaseServersByFiltersForCluster(ctx, req, proxy.siteName)
	return servers, trace.Wrap(err)
}

// FindDatabaseServersByFiltersForCluster returns all registered database proxy servers in the provided cluster.
func (proxy *ProxyClient) FindDatabaseServersByFiltersForCluster(ctx context.Context, req proto.ListResourcesRequest, cluster string) ([]types.DatabaseServer, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/FindDatabaseServersByFiltersForCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", cluster),
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	authClient, err := proxy.ConnectToCluster(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := client.GetAllResources[types.DatabaseServer](ctx, authClient, &req)
	return servers, trace.Wrap(err)
}

// FindDatabasesByFilters returns registered databases that match the provided
// filter in the current cluster.
func (proxy *ProxyClient) FindDatabasesByFilters(ctx context.Context, req proto.ListResourcesRequest) ([]types.Database, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/FindDatabasesByFilters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	databases, err := proxy.FindDatabasesByFiltersForCluster(ctx, req, proxy.siteName)
	return databases, trace.Wrap(err)
}

// FindDatabasesByFiltersForCluster returns registered databases that match the provided
// filter in the provided cluster.
func (proxy *ProxyClient) FindDatabasesByFiltersForCluster(ctx context.Context, req proto.ListResourcesRequest, cluster string) ([]types.Database, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/FindDatabasesByFiltersForCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	servers, err := proxy.FindDatabaseServersByFiltersForCluster(ctx, req, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	databases := types.DatabaseServers(servers).ToDatabases()
	return types.DeduplicateDatabases(databases), nil
}

// ListResources returns a paginated list of resources.
func (proxy *ProxyClient) ListResources(ctx context.Context, namespace, resource, startKey string, limit int) ([]types.ResourceWithLabels, string, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/ListResources",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("resource", resource),
			attribute.Int("limit", limit),
		),
	)
	defer span.End()

	authClient := proxy.CurrentCluster()

	resp, err := authClient.ListResources(ctx, proto.ListResourcesRequest{
		Namespace:    namespace,
		ResourceType: resource,
		StartKey:     startKey,
		Limit:        int32(limit),
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp.Resources, resp.NextKey, nil
}

// sharedAuthClient is a wrapper around auth.ClientI which
// prevents the underlying client from being closed.
type sharedAuthClient struct {
	authclient.ClientI
}

// Close is a no-op
func (a sharedAuthClient) Close() error {
	return nil
}

// CurrentCluster returns an authenticated auth server client for the local cluster.
func (proxy *ProxyClient) CurrentCluster() authclient.ClientI {
	// The auth.ClientI is wrapped in an sharedAuthClient to prevent callers from
	// being able to close the client. The auth.ClientI is only to be closed
	// when the ProxyClient is closed.
	return sharedAuthClient{ClientI: proxy.currentCluster}
}

// ConnectToRootCluster connects to the auth server of the root cluster
// via proxy. It returns connected and authenticated auth server client
func (proxy *ProxyClient) ConnectToRootCluster(ctx context.Context) (authclient.ClientI, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/ConnectToRootCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	clusterName, err := proxy.RootClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return proxy.ConnectToCluster(ctx, clusterName)
}

func (proxy *ProxyClient) loadTLS(clusterName string) (*tls.Config, error) {
	return proxy.teleportClient.LoadTLSConfigForClusters([]string{clusterName})
}

// ConnectToAuthServiceThroughALPNSNIProxy uses ALPN proxy service to connect to remote/local auth
// service and returns auth client. For routing purposes, TLS ServerName is set to destination auth service
// cluster name with ALPN values set to teleport-auth protocol.
func (proxy *ProxyClient) ConnectToAuthServiceThroughALPNSNIProxy(ctx context.Context, clusterName, proxyAddr string) (authclient.ClientI, error) {
	tlsConfig, err := proxy.loadTLS(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if proxyAddr == "" {
		proxyAddr = proxy.teleportClient.WebProxyAddr
	}

	tlsConfig.InsecureSkipVerify = proxy.teleportClient.InsecureSkipVerify
	clt, err := authclient.NewClient(client.Config{
		Context: ctx,
		Addrs:   []string{proxyAddr},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		ALPNSNIAuthDialClusterName: clusterName,
		CircuitBreakerConfig:       breaker.NoopBreakerConfig(),
		ALPNConnUpgradeRequired:    proxy.teleportClient.IsALPNConnUpgradeRequiredForWebProxy(ctx, proxyAddr),
		PROXYHeaderGetter:          CreatePROXYHeaderGetter(ctx, proxy.teleportClient.PROXYSigner),
		InsecureAddressDiscovery:   proxy.teleportClient.InsecureSkipVerify,
		MFAPromptConstructor:       proxy.teleportClient.NewMFAPrompt,
		DialOpts:                   proxy.teleportClient.DialOpts,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

func (proxy *ProxyClient) shouldDialWithTLSRouting(ctx context.Context) (string, bool) {
	if len(proxy.teleportClient.JumpHosts) > 0 {
		// Check if the provided JumpHost address is a Teleport Proxy.
		// This is needed to distinguish if the JumpHost address from Teleport Proxy Web address
		// or Teleport Proxy SSH address.
		jumpHostAddr := proxy.teleportClient.JumpHosts[0].Addr.String()
		resp, err := webclient.Find(
			&webclient.Config{
				Context:   ctx,
				ProxyAddr: jumpHostAddr,
				Insecure:  proxy.teleportClient.InsecureSkipVerify,
			},
		)
		if err != nil {
			// HTTP ping call failed. The JumpHost address is not a Teleport proxy address
			return "", false
		}
		return jumpHostAddr, resp.Proxy.TLSRoutingEnabled
	}
	return proxy.teleportClient.WebProxyAddr, proxy.teleportClient.TLSRoutingEnabled
}

// ConnectToCluster connects to the auth server of the given cluster via proxy.
// It returns connected and authenticated auth server client
func (proxy *ProxyClient) ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	// If connecting to the local cluster then return the already
	// established auth client instead of dialing it a second time.
	if clusterName == proxy.siteName && proxy.currentCluster != nil {
		return proxy.CurrentCluster(), nil
	}

	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/ConnectToCluster",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", clusterName),
		),
	)
	defer span.End()

	if proxyAddr, ok := proxy.shouldDialWithTLSRouting(ctx); ok {
		// If proxy supports multiplex listener mode dial root/leaf cluster auth service via ALPN Proxy
		// directly without using SSH tunnels.
		clt, err := proxy.ConnectToAuthServiceThroughALPNSNIProxy(ctx, clusterName, proxyAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clt, nil
	}

	dialer := client.ContextDialerFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
		// link the span created dialing the auth server to the one created above. grpc dialing
		// passes in a context.Background() during dial which causes these two spans to be in
		// different traces.
		ctx = oteltrace.ContextWithSpan(ctx, span)
		return proxy.dialAuthServer(ctx, clusterName)
	})

	tlsConfig, err := proxy.loadTLS(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := authclient.NewClient(client.Config{
		Context: ctx,
		Dialer:  dialer,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		CircuitBreakerConfig: breaker.NoopBreakerConfig(),
		MFAPromptConstructor: proxy.teleportClient.NewMFAPrompt,
		DialOpts:             proxy.teleportClient.DialOpts,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// NewTracingClient connects to the auth server of the given cluster via proxy.
// It returns a connected and authenticated tracing.Client that will export spans
// to the auth server, where they will be forwarded onto the configured exporter.
func (proxy *ProxyClient) NewTracingClient(ctx context.Context, clusterName string) (*tracing.Client, error) {
	tlsConfig, err := proxy.loadTLS(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientConfig := client.Config{
		DialInBackground: true,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	}

	switch {
	case proxy.teleportClient.TLSRoutingEnabled:
		clientConfig.Addrs = []string{proxy.teleportClient.WebProxyAddr}
		clientConfig.ALPNSNIAuthDialClusterName = clusterName
		clientConfig.ALPNConnUpgradeRequired = proxy.teleportClient.TLSRoutingConnUpgradeRequired
	default:
		clientConfig.Dialer = client.ContextDialerFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
			return proxy.dialAuthServer(ctx, clusterName)
		})
	}

	clt, err := client.NewTracingClient(ctx, clientConfig)
	return clt, trace.Wrap(err)
}

// nodeName removes the port number from the hostname, if present
func nodeName(node targetNode) string {
	if node.hostname != "" {
		return node.hostname
	}
	n, _, err := net.SplitHostPort(node.addr)
	if err != nil {
		return node.addr
	}
	return n
}

// dialAuthServer returns auth server connection forwarded via proxy
func (proxy *ProxyClient) dialAuthServer(ctx context.Context, clusterName string) (net.Conn, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/dialAuthServer",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", clusterName),
		),
	)
	defer span.End()

	log.Debugf("Client %v is connecting to auth server on cluster %q.", proxy.clientAddr, clusterName)

	address := "@" + clusterName

	// parse destination first:
	localAddr, err := utils.ParseAddr("tcp://" + proxy.proxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fakeAddr, err := utils.ParseAddr("tcp://" + address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxySession, err := proxy.Client.NewSession(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyWriter, err := proxySession.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyReader, err := proxySession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyErr, err := proxySession.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = proxySession.RequestSubsystem(ctx, "proxy:"+address)
	if err != nil {
		// read the stderr output from the failed SSH session and append
		// it to the end of our own message:
		serverErrorMsg, _ := io.ReadAll(proxyErr)
		return nil, trace.ConnectionProblem(err, "failed connecting to node %v. %s",
			nodeName(targetNode{addr: strings.Split(address, "@")[0]}), serverErrorMsg)
	}
	return utils.NewPipeNetConn(
		proxyReader,
		proxyWriter,
		proxySession,
		localAddr,
		fakeAddr,
	), nil
}

// NodeDetails provides connection information for a node
type NodeDetails struct {
	// Addr is an address to dial
	Addr string
	// Namespace is the node namespace
	Namespace string
	// Cluster is the name of the target cluster
	Cluster string

	// MFACheck is optional parameter passed if MFA check was already done.
	// It can be nil.
	MFACheck *proto.IsMFARequiredResponse

	// hostname is the node's hostname, for more user-friendly logging.
	hostname string
}

// String returns a user-friendly name
func (n NodeDetails) String() string {
	parts := []string{nodeName(targetNode{addr: n.Addr})}
	if n.Cluster != "" {
		parts = append(parts, "on cluster", n.Cluster)
	}
	return strings.Join(parts, " ")
}

// ProxyFormat returns the address in the format
// used by the proxy subsystem
func (n *NodeDetails) ProxyFormat() string {
	parts := []string{n.Addr}
	if n.Namespace != "" {
		parts = append(parts, n.Namespace)
	}
	if n.Cluster != "" {
		parts = append(parts, n.Cluster)
	}
	return strings.Join(parts, "@")
}

// requestSubsystem sends a subsystem request on the session. If the passed
// in context is canceled first, unblocks.
func requestSubsystem(ctx context.Context, session *tracessh.Session, name string) error {
	errCh := make(chan error)

	go func() {
		er := session.RequestSubsystem(ctx, name)
		errCh <- er
	}()

	select {
	case err := <-errCh:
		return trace.Wrap(err)
	case <-ctx.Done():
		err := session.Close()
		if err != nil {
			log.Debugf("Failed to close session: %v.", err)
		}
		return trace.Wrap(ctx.Err())
	}
}

// ConnectToNode connects to the ssh server via Proxy.
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) ConnectToNode(ctx context.Context, nodeAddress NodeDetails, user string, details sshutils.ClusterDetails) (*NodeClient, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/ConnectToNode",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("node", nodeAddress.Addr),
			attribute.String("cluster", nodeAddress.Cluster),
			attribute.String("user", user),
		),
	)
	defer span.End()

	log.Infof("Client=%v connecting to node=%v", proxy.clientAddr, nodeAddress)
	if len(proxy.teleportClient.JumpHosts) > 0 {
		return proxy.PortForwardToNode(ctx, nodeAddress, user, details, proxy.authMethods)
	}

	// parse destination first:
	localAddr, err := utils.ParseAddr("tcp://" + proxy.proxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fakeAddr, err := utils.ParseAddr("tcp://" + nodeAddress.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxySession, err := proxy.Client.NewSession(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyWriter, err := proxySession.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyReader, err := proxySession.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyErr, err := proxySession.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// the client only tries to forward an agent when the proxy is in recording
	// mode. we always try and forward an agent here because each new session
	// creates a new context which holds the agent. if ForwardToAgent returns an error
	// "already have handler for" we ignore it.
	if details.RecordingProxy {
		if proxy.teleportClient.localAgent == nil {
			return nil, trace.BadParameter("cluster is in proxy recording mode and requires agent forwarding for connections, but no agent was initialized")
		}
		err = agent.ForwardToAgent(proxy.Client.Client, proxy.teleportClient.localAgent.ExtendedAgent)
		if err != nil && !strings.Contains(err.Error(), "agent: already have handler for") {
			return nil, trace.Wrap(err)
		}

		err = agent.RequestAgentForwarding(proxySession.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	err = requestSubsystem(ctx, proxySession, "proxy:"+nodeAddress.ProxyFormat())
	if err != nil {
		// If the user pressed Ctrl-C, no need to try and read the error from
		// the proxy, return an error right away.
		if trace.Unwrap(err) == context.Canceled {
			return nil, trace.Wrap(err)
		}

		// read the stderr output from the failed SSH session and append
		// it to the end of our own message:
		serverErrorMsg, _ := io.ReadAll(proxyErr)
		return nil, trace.ConnectionProblem(err, "failed connecting to node %v. %s",
			nodeName(targetNode{addr: nodeAddress.Addr}), serverErrorMsg)
	}

	pipeNetConn := utils.NewPipeNetConn(
		proxyReader,
		proxyWriter,
		proxySession,
		localAddr,
		fakeAddr,
	)

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            proxy.authMethods,
		HostKeyCallback: proxy.hostKeyCallback,
	}

	nc, err := NewNodeClient(ctx, sshConfig, pipeNetConn,
		nodeAddress.ProxyFormat(), nodeAddress.Addr,
		proxy.teleportClient, details.FIPSEnabled)
	return nc, trace.Wrap(err)
}

// PortForwardToNode connects to the ssh server via Proxy
// It returns connected and authenticated NodeClient
func (proxy *ProxyClient) PortForwardToNode(ctx context.Context, nodeAddress NodeDetails, user string, details sshutils.ClusterDetails, authMethods []ssh.AuthMethod) (*NodeClient, error) {
	ctx, span := proxy.Tracer.Start(
		ctx,
		"proxyClient/PortForwardToNode",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("node", nodeAddress.Addr),
			attribute.String("cluster", nodeAddress.Cluster),
			attribute.String("user", user),
		),
	)
	defer span.End()

	log.Infof("Client=%v jumping to node=%s", proxy.clientAddr, nodeAddress)

	// the client only tries to forward an agent when the proxy is in recording
	// mode. we always try and forward an agent here because each new session
	// creates a new context which holds the agent. if ForwardToAgent returns an error
	// "already have handler for" we ignore it.
	if details.RecordingProxy {
		if proxy.teleportClient.localAgent == nil {
			return nil, trace.BadParameter("cluster is in proxy recording mode and requires agent forwarding for connections, but no agent was initialized")
		}
		err := agent.ForwardToAgent(proxy.Client.Client, proxy.teleportClient.localAgent.ExtendedAgent)
		if err != nil && !strings.Contains(err.Error(), "agent: already have handler for") {
			return nil, trace.Wrap(err)
		}
	}

	proxyConn, err := proxy.Client.DialContext(ctx, "tcp", nodeAddress.Addr)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "failed connecting to node %v. %s", nodeAddress, err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: proxy.hostKeyCallback,
	}

	nc, err := NewNodeClient(ctx, sshConfig, proxyConn, nodeAddress.Addr, "", proxy.teleportClient, details.FIPSEnabled)
	return nc, trace.Wrap(err)
}

// NodeClientOption is a functional argument for NewNodeClient.
type NodeClientOption func(nc *NodeClient)

// WithNodeHostname sets the hostname to display for the connected node.
func WithNodeHostname(hostname string) NodeClientOption {
	return func(nc *NodeClient) {
		nc.hostname = hostname
	}
}

// WithSSHLogDir sets the directory to write command output to when running
// commands on multiple nodes.
func WithSSHLogDir(logDir string) NodeClientOption {
	return func(nc *NodeClient) {
		nc.sshLogDir = logDir
	}
}

// NewNodeClient constructs a NodeClient that is connected to the node at nodeAddress.
// The nodeName field is optional and is used only to present better error messages.
func NewNodeClient(ctx context.Context, sshConfig *ssh.ClientConfig, conn net.Conn, nodeAddress, nodeName string, tc *TeleportClient, fipsEnabled bool, opts ...NodeClientOption) (*NodeClient, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"NewNodeClient",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("node", nodeAddress),
		),
	)
	defer span.End()

	if nodeName == "" {
		nodeName = nodeAddress
	}

	sshconn, chans, reqs, err := newClientConn(ctx, conn, nodeAddress, sshConfig)
	if err != nil {
		if utils.IsHandshakeFailedError(err) {
			conn.Close()
			// TODO(codingllama): Improve error message below for device trust.
			//  An alternative we have here is querying the cluster to check if device
			//  trust is required, a check similar to `IsMFARequired`.
			log.Infof("Access denied to %v connecting to %v: %v", sshConfig.User, nodeName, err)
			return nil, trace.AccessDenied(`access denied to %v connecting to %v`, sshConfig.User, nodeName)
		}
		return nil, trace.Wrap(err)
	}

	// We pass an empty channel which we close right away to ssh.NewClient
	// because the client need to handle requests itself.
	emptyCh := make(chan *ssh.Request)
	close(emptyCh)

	nc := &NodeClient{
		Client:          tracessh.NewClient(sshconn, chans, emptyCh),
		Namespace:       apidefaults.Namespace,
		TC:              tc,
		Tracer:          tc.Tracer,
		FIPSEnabled:     fipsEnabled,
		ProxyPublicAddr: tc.WebProxyAddr,
		hostname:        nodeName,
	}

	for _, opt := range opts {
		opt(nc)
	}

	// Start a goroutine that will run for the duration of the client to process
	// global requests from the client. Teleport clients will use this to update
	// terminal sizes when the remote PTY size has changed.
	go nc.handleGlobalRequests(ctx, reqs)

	return nc, nil
}

// RunInteractiveShell creates an interactive shell on the node and copies stdin/stdout/stderr
// to and from the node and local shell. This will block until the interactive shell on the node
// is terminated.
func (c *NodeClient) RunInteractiveShell(ctx context.Context, mode types.SessionParticipantMode, sessToJoin types.SessionTracker, beforeStart func(io.Writer)) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/RunInteractiveShell",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	env := c.TC.newSessionEnv()
	env[teleport.EnvSSHJoinMode] = string(mode)
	env[teleport.EnvSSHSessionReason] = c.TC.Config.Reason
	env[teleport.EnvSSHSessionDisplayParticipantRequirements] = strconv.FormatBool(c.TC.Config.DisplayParticipantRequirements)
	encoded, err := json.Marshal(&c.TC.Config.Invited)
	if err != nil {
		return trace.Wrap(err)
	}
	env[teleport.EnvSSHSessionInvited] = string(encoded)

	// Overwrite "SSH_SESSION_WEBPROXY_ADDR" with the public addr reported by the proxy. Otherwise,
	// this would be set to the localhost addr (tc.WebProxyAddr) used for Web UI client connections.
	if c.ProxyPublicAddr != "" && c.TC.WebProxyAddr != c.ProxyPublicAddr {
		env[teleport.SSHSessionWebProxyAddr] = c.ProxyPublicAddr
	}

	nodeSession, err := newSession(ctx, c, sessToJoin, env, c.TC.Stdin, c.TC.Stdout, c.TC.Stderr, c.TC.EnableEscapeSequences)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = nodeSession.runShell(ctx, mode, beforeStart, c.TC.OnShellCreated); err != nil {
		switch e := trace.Unwrap(err).(type) {
		case *ssh.ExitError:
			c.TC.ExitStatus = e.ExitStatus()
		case *ssh.ExitMissingError:
			c.TC.ExitStatus = 1
		}

		return trace.Wrap(err)
	}

	if nodeSession.ExitMsg == "" {
		fmt.Fprintln(c.TC.Stderr, "the connection was closed on the remote side at ", time.Now().Format(time.RFC822))
	} else {
		fmt.Fprintln(c.TC.Stderr, nodeSession.ExitMsg)
	}

	return nil
}

// lineLabeledWriter is an io.Writer that prepends a label to each line it writes.
type lineLabeledWriter struct {
	linePrefix        []byte
	w                 io.Writer
	shouldWritePrefix bool
}

func newLineLabeledWriter(w io.Writer, label string) io.Writer {
	return &lineLabeledWriter{
		linePrefix:        []byte(fmt.Sprintf("[%v] ", label)),
		w:                 w,
		shouldWritePrefix: true,
	}
}

func (lw *lineLabeledWriter) writeChunk(b []byte, bytesWritten int, newline bool) (int, error) {
	n, err := lw.w.Write(b)
	bytesWritten += n
	if err != nil {
		return bytesWritten, trace.Wrap(err)
	}
	if newline {
		n, err = lw.w.Write([]byte("\n"))
		bytesWritten += n
	}
	return bytesWritten, trace.Wrap(err)
}

func (lw *lineLabeledWriter) Write(input []byte) (int, error) {
	bytesWritten := 0
	var line []byte
	rest := input
	var found bool
	for {
		line, rest, found = bytes.Cut(rest, []byte("\n"))
		// Write the prefix unless we're either continuing a line from the last
		// write or we're at the end.
		if lw.shouldWritePrefix && (len(line) > 0 || found) {
			// Write the prefix on its own to not mess with the eventual returned
			// number of bytes written.
			if _, err := lw.w.Write(lw.linePrefix); err != nil {
				return bytesWritten, trace.Wrap(err)
			}
		}
		var err error
		if bytesWritten, err = lw.writeChunk(line, bytesWritten, found); err != nil {
			return bytesWritten, trace.Wrap(err)
		}
		lw.shouldWritePrefix = true

		if !found {
			// If there were leftovers, the line will continue on the next write, so
			// skip the first prefix next time.
			lw.shouldWritePrefix = len(line) == 0
			break
		}
	}

	return bytesWritten, nil
}

// RunCommandOptions is a set of options for NodeClient.RunCommand.
type RunCommandOptions struct {
	labelLines bool
}

// RunCommandOption is a functional argument for NodeClient.RunCommand.
type RunCommandOption func(opts *RunCommandOptions)

// WithLabeledOutput labels each line of output from a command with the node's
// hostname.
func WithLabeledOutput() RunCommandOption {
	return func(opts *RunCommandOptions) {
		opts.labelLines = true
	}
}

// RunCommand executes a given bash command on the node.
func (c *NodeClient) RunCommand(ctx context.Context, command []string, opts ...RunCommandOption) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/RunCommand",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	var options RunCommandOptions
	for _, opt := range opts {
		opt(&options)
	}

	// Set up output streams
	stdout := c.TC.Stdout
	stderr := c.TC.Stderr
	if c.hostname != "" {
		if options.labelLines {
			stdout = newLineLabeledWriter(c.TC.Stdout, c.hostname)
			stderr = newLineLabeledWriter(c.TC.Stderr, c.hostname)
		}

		if c.sshLogDir != "" {
			stdoutFile, err := os.Create(filepath.Join(c.sshLogDir, c.hostname+".stdout"))
			if err != nil {
				return trace.Wrap(err)
			}
			defer stdoutFile.Close()
			stderrFile, err := os.Create(filepath.Join(c.sshLogDir, c.hostname+".stderr"))
			if err != nil {
				return trace.Wrap(err)
			}
			defer stderrFile.Close()

			stdout = io.MultiWriter(stdout, stdoutFile)
			stderr = io.MultiWriter(stderr, stderrFile)
		}
	}

	nodeSession, err := newSession(ctx, c, nil, c.TC.newSessionEnv(), c.TC.Stdin, stdout, stderr, c.TC.EnableEscapeSequences)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nodeSession.Close()
	if err := nodeSession.runCommand(ctx, types.SessionPeerMode, command, c.TC.OnShellCreated, c.TC.Config.InteractiveCommand); err != nil {
		originErr := trace.Unwrap(err)
		exitErr, ok := originErr.(*ssh.ExitError)
		if ok {
			c.TC.ExitStatus = exitErr.ExitStatus()
		} else {
			// if an error occurs, but no exit status is passed back, GoSSH returns
			// a generic error like this. in this case the error message is printed
			// to stderr by the remote process so we have to quietly return 1:
			if strings.Contains(originErr.Error(), "exited without exit status") {
				c.TC.ExitStatus = 1
			}
		}

		return trace.Wrap(err)
	}

	return nil
}

// AddEnv add environment variable to SSH session. This method needs to be called
// before the session is created.
func (c *NodeClient) AddEnv(key, value string) {
	if c.TC.extraEnvs == nil {
		c.TC.extraEnvs = make(map[string]string)
	}
	c.TC.extraEnvs[key] = value
}

func (c *NodeClient) handleGlobalRequests(ctx context.Context, requestCh <-chan *ssh.Request) {
	for {
		select {
		case r := <-requestCh:
			// When the channel is closing, nil is returned.
			if r == nil {
				return
			}

			switch r.Type {
			case teleport.MFAPresenceRequest:
				if c.OnMFA == nil {
					log.Warn("Received MFA presence request, but no callback was provided.")
					continue
				}

				go c.OnMFA()
			case teleport.SessionEvent:
				// Parse event and create events.EventFields that can be consumed directly
				// by caller.
				var e events.EventFields
				err := json.Unmarshal(r.Payload, &e)
				if err != nil {
					log.Warnf("Unable to parse event: %v: %v.", string(r.Payload), err)
					continue
				}

				// Send event to event channel.
				err = c.TC.SendEvent(ctx, e)
				if err != nil {
					log.Warnf("Unable to send event %v: %v.", string(r.Payload), err)
					continue
				}
			default:
				// This handles keep-alive messages and matches the behavior of OpenSSH.
				err := r.Reply(false, nil)
				if err != nil {
					log.Warnf("Unable to reply to %v request.", r.Type)
					continue
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// newClientConn is a wrapper around ssh.NewClientConn
func newClientConn(
	ctx context.Context,
	conn net.Conn,
	nodeAddress string,
	config *ssh.ClientConfig,
) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	type response struct {
		conn   ssh.Conn
		chanCh <-chan ssh.NewChannel
		reqCh  <-chan *ssh.Request
		err    error
	}

	respCh := make(chan response, 1)
	go func() {
		// Use a noop text map propagator so that the tracing context isn't included in
		// the connection handshake. Since the provided conn will already include the tracing
		// context we don't want to send it again.
		conn, chans, reqs, err := tracessh.NewClientConn(ctx, conn, nodeAddress, config, tracing.WithTextMapPropagator(propagation.NewCompositeTextMapPropagator()))
		respCh <- response{conn, chans, reqs, err}
	}()

	select {
	case resp := <-respCh:
		if resp.err != nil {
			return nil, nil, nil, trace.Wrap(resp.err, "failed to connect to %q", nodeAddress)
		}
		return resp.conn, resp.chanCh, resp.reqCh, nil
	case <-ctx.Done():
		errClose := conn.Close()
		if errClose != nil {
			log.Error(errClose)
		}
		// drain the channel
		resp := <-respCh
		return nil, nil, nil, trace.ConnectionProblem(resp.err, "failed to connect to %q", nodeAddress)
	}
}

// Close closes the proxy and auth clients
func (proxy *ProxyClient) Close() error {
	return trace.NewAggregate(proxy.Client.Close(), proxy.currentCluster.Close())
}

// TransferFiles transfers files over SFTP.
func (c *NodeClient) TransferFiles(ctx context.Context, cfg *sftp.Config) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/TransferFiles",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	return trace.Wrap(cfg.TransferFiles(ctx, c.Client.Client))
}

type netDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

func proxyConnection(ctx context.Context, conn net.Conn, remoteAddr string, dialer netDialer) error {
	defer conn.Close()
	defer log.Debugf("Finished proxy from %v to %v.", conn.RemoteAddr(), remoteAddr)

	var remoteConn net.Conn
	log.Debugf("Attempting to connect proxy from %v to %v.", conn.RemoteAddr(), remoteAddr)

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  100 * time.Millisecond,
		Step:   100 * time.Millisecond,
		Max:    time.Second,
		Jitter: retryutils.NewHalfJitter(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		conn, err := dialer.DialContext(ctx, "tcp", remoteAddr)
		if err == nil {
			// Connection established, break out of the loop.
			remoteConn = conn
			break
		}

		log.Debugf("Proxy connection attempt %v: %v.", attempt, err)
		// Wait and attempt to connect again, if the context has closed, exit
		// right away.
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-retry.After():
			retry.Inc()
			continue
		}
	}
	if remoteConn == nil {
		return trace.BadParameter("failed to connect to node: %v", remoteAddr)
	}
	defer remoteConn.Close()

	// Start proxying, close the connection if a problem occurs on either leg.
	return trace.Wrap(utils.ProxyConn(ctx, remoteConn, conn))
}

// acceptWithContext calls "Accept" on the listener but will unblock when the
// context is canceled.
func acceptWithContext(ctx context.Context, l net.Listener) (net.Conn, error) {
	acceptCh := make(chan net.Conn, 1)
	errorCh := make(chan error, 1)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			errorCh <- err
			return
		}
		acceptCh <- conn
	}()

	select {
	case conn := <-acceptCh:
		return conn, nil
	case err := <-errorCh:
		return nil, trace.Wrap(err)
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	}
}

// listenAndForward listens on a given socket and forwards all incoming
// commands to the remote address through the SSH tunnel.
func (c *NodeClient) listenAndForward(ctx context.Context, ln net.Listener, localAddr string, remoteAddr string) {
	defer ln.Close()

	log := log.WithField("localAddr", localAddr).WithField("remoteAddr", remoteAddr)

	log.Infof("Starting port forwarding")

	for ctx.Err() == nil {
		// Accept connections from the client.
		conn, err := acceptWithContext(ctx, ln)
		if err != nil {
			if ctx.Err() == nil {
				log.WithError(err).Errorf("Port forwarding failed.")
			}
			continue
		}

		// Proxy the connection to the remote address.
		go func() {
			// `err` must be a fresh variable, hence `:=` instead of `=`.
			if err := proxyConnection(ctx, conn, remoteAddr, c.Client); err != nil {
				log.WithError(err).Warnf("Failed to proxy connection.")
			}
		}()
	}

	log.WithError(ctx.Err()).Infof("Shutting down port forwarding.")
}

// dynamicListenAndForward listens for connections, performs a SOCKS5
// handshake, and then proxies the connection to the requested address.
func (c *NodeClient) dynamicListenAndForward(ctx context.Context, ln net.Listener, localAddr string) {
	defer ln.Close()

	log := log.WithField("localAddr", localAddr)

	log.Infof("Starting dynamic port forwarding.")

	for ctx.Err() == nil {
		// Accept connection from the client. Here the client is typically
		// something like a web browser or other SOCKS5 aware application.
		conn, err := acceptWithContext(ctx, ln)
		if err != nil {
			if ctx.Err() == nil {
				log.WithError(err).Errorf("Dynamic port forwarding (SOCKS5) failed.")
			}
			continue
		}

		// Perform the SOCKS5 handshake with the client to find out the remote
		// address to proxy.
		remoteAddr, err := socks.Handshake(conn)
		if err != nil {
			log.WithError(err).Errorf("SOCKS5 handshake failed.")
			if err = conn.Close(); err != nil {
				log.WithError(err).Errorf("Error closing failed proxy connection.")
			}
			continue
		}
		log.Debugf("SOCKS5 proxy forwarding requests to %v.", remoteAddr)

		// Proxy the connection to the remote address.
		go func() {
			// `err` must be a fresh variable, hence `:=` instead of `=`.
			if err := proxyConnection(ctx, conn, remoteAddr, c.Client); err != nil {
				log.WithError(err).Warnf("Failed to proxy connection.")
				if err = conn.Close(); err != nil {
					log.WithError(err).Errorf("Error closing failed proxy connection.")
				}
			}
		}()
	}

	log.WithError(ctx.Err()).Infof("Shutting down dynamic port forwarding.")
}

// remoteListenAndForward requests a listening socket and forwards all incoming
// commands to the local address through the SSH tunnel.
func (c *NodeClient) remoteListenAndForward(ctx context.Context, ln net.Listener, localAddr, remoteAddr string) {
	defer ln.Close()
	log := log.WithField("localAddr", localAddr).WithField("remoteAddr", remoteAddr)
	log.Infof("Starting remote port forwarding")

	for ctx.Err() == nil {
		conn, err := acceptWithContext(ctx, ln)
		if err != nil {
			if ctx.Err() == nil {
				log.WithError(err).Errorf("Remote port forwarding failed.")
			}
			continue
		}

		go func() {
			if err := proxyConnection(ctx, conn, localAddr, &net.Dialer{}); err != nil {
				log.WithError(err).Warnf("Failed to proxy connection")
			}
		}()
	}
	log.WithError(ctx.Err()).Infof("Shutting down remote port forwarding.")
}

// GetRemoteTerminalSize fetches the terminal size of a given SSH session.
func (c *NodeClient) GetRemoteTerminalSize(ctx context.Context, sessionID string) (*term.Winsize, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"nodeClient/GetRemoteTerminalSize",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("session", sessionID)),
	)
	defer span.End()

	ok, payload, err := c.Client.SendRequest(ctx, teleport.TerminalSizeRequest, true, []byte(sessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	} else if !ok {
		return nil, trace.BadParameter("failed to get terminal size")
	}

	ws := new(term.Winsize)
	err = json.Unmarshal(payload, ws)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ws, nil
}

// Close closes client and it's operations
func (c *NodeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errors []error
	for _, closer := range c.closers {
		errors = append(errors, closer.Close())
	}

	c.closers = nil

	errors = append(errors, c.Client.Close())

	return trace.NewAggregate(errors...)
}

// localAgent returns for the Teleport client's local agent.
func (proxy *ProxyClient) localAgent() *LocalKeyAgent {
	return proxy.teleportClient.LocalAgent()
}

// GetPaginatedSessions grabs up to 'max' sessions.
func GetPaginatedSessions(ctx context.Context, fromUTC, toUTC time.Time, pageSize int, order types.EventOrder, max int, authClient authclient.ClientI) ([]apievents.AuditEvent, error) {
	prevEventKey := ""
	var sessions []apievents.AuditEvent
	for {
		if remaining := max - len(sessions); remaining < pageSize {
			pageSize = remaining
		}
		nextEvents, eventKey, err := authClient.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
			From:     fromUTC,
			To:       toUTC,
			Limit:    pageSize,
			Order:    order,
			StartKey: prevEventKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sessions = append(sessions, nextEvents...)
		if eventKey == "" || len(sessions) >= max {
			break
		}
		prevEventKey = eventKey
	}
	if max < len(sessions) {
		return sessions[:max], nil
	}
	return sessions, nil
}
