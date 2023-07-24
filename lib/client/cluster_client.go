// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
)

// ClusterClient facilitates communicating with both the
// Auth and Proxy services of a cluster.
type ClusterClient struct {
	*proxyclient.Client
	tc         *TeleportClient
	authClient auth.ClientI

	cluster string
	root    string

	Tracer oteltrace.Tracer
}

// ClusterName returns the name of the cluster that the client
// is connected to.
func (c *ClusterClient) ClusterName() string {
	return c.cluster
}

// CurrentCluster returns an authenticated auth server client for the local cluster.
func (c *ClusterClient) CurrentCluster() auth.ClientI {
	// The auth.ClientI is wrapped in an sharedAuthClient to prevent callers from
	// being able to close the client. The auth.ClientI is only to be closed
	// when the ProxyClient is closed.
	return sharedAuthClient{ClientI: c.authClient}
}

func (c *ClusterClient) RootCluster(ctx context.Context) (auth.ClientI, error) {
	root, err := c.ConnectToCluster(ctx, c.root)
	return root, trace.Wrap(err)
}

// ConnectToCluster connects to the auth server of the given cluster via proxy. It returns connected and authenticated auth server client
func (c *ClusterClient) ConnectToCluster(ctx context.Context, clusterName string) (auth.ClientI, error) {
	if c.cluster == clusterName {
		return c.CurrentCluster(), nil
	}

	clientConfig := c.ClientConfig(ctx, clusterName)
	authClient, err := auth.NewClient(clientConfig)
	return authClient, trace.Wrap(err)
}

// Close terminates the connections to Auth and Proxy.
func (c *ClusterClient) Close() error {
	// close auth client first since it is tunneled through the proxy client
	return trace.NewAggregate(c.authClient.Close(), c.Client.Close())
}

// SessionSSHConfig returns the [ssh.ClientConfig] that should be used to connected to the
// provided target for the provided user. If per session MFA is required to establish the
// connection, then the MFA ceremony will be performed.
func (c *ClusterClient) SessionSSHConfig(ctx context.Context, user string, target NodeDetails) (*ssh.ClientConfig, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/SessionSSHConfig",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", c.tc.SiteName),
		),
	)
	defer span.End()

	sshConfig := c.Client.SSHConfig(user)

	if target.MFACheck != nil && !target.MFACheck.Required {
		return sshConfig, nil
	}

	key, err := c.tc.localAgent.GetKey(target.Cluster, WithAllCerts...)
	if err != nil {
		return nil, trace.Wrap(MFARequiredUnknown(err))
	}

	// Always connect to root for getting new credentials, but attempt to reuse
	// the existing client if possible.
	mfaClt := c
	if target.Cluster != c.root {
		aclt, err := auth.NewClient(c.Client.ClientConfig(ctx, c.root))
		if err != nil {
			return nil, trace.Wrap(MFARequiredUnknown(err))
		}

		mfaClt = &ClusterClient{
			Client:     c.Client,
			tc:         c.tc,
			authClient: aclt,
			Tracer:     c.Tracer,
			cluster:    c.root,
			root:       c.root,
		}
		// only close the new auth client and not the copied cluster client.
		defer aclt.Close()
	}

	log.Debug("Attempting to issue a single-use user certificate with an MFA check.")
	key, err = c.performMFACeremony(ctx, mfaClt,
		ReissueParams{
			NodeName:       nodeName(target.Addr),
			RouteToCluster: target.Cluster,
			MFACheck:       target.MFACheck,
		},
		key,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Debug("Issued single-use user certificate after an MFA check.")
	am, err := key.AsAuthMethod()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig.Auth = []ssh.AuthMethod{am}
	return sshConfig, nil
}

// ReissueUserCerts gets new user certificates from the root Auth server.
func (c *ClusterClient) ReissueUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/reissueUserCerts",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", c.tc.SiteName),
		),
	)
	defer span.End()

	key, err := c.reissueUserCerts(ctx, cachePolicy, params)
	if err != nil {
		return trace.Wrap(err)
	}

	if cachePolicy == CertCacheDrop {
		c.tc.LocalAgent().DeleteUserCerts("", WithAllCerts...)
	}

	// save the cert to the local storage (~/.tsh usually):
	err = c.tc.LocalAgent().AddKey(key)
	return trace.Wrap(err)
}

func (c *ClusterClient) reissueUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) (*Key, error) {
	if params.RouteToCluster == "" {
		params.RouteToCluster = c.tc.SiteName
	}
	key := params.ExistingCreds
	if key == nil {
		var err error

		// Don't load the certs if we're going to drop all of them all as part
		// of the re-issue. If we load all of the old certs now we won't be able
		// to differentiate between legacy certificates (that need to be
		// deleted) and newly re-issued certs (that we definitely do *not* want
		// to delete) when it comes time to drop them from the local agent.
		var certOptions []CertOption
		if cachePolicy == CertCacheKeep {
			certOptions = WithAllCerts
		}

		key, err = c.tc.localAgent.GetKey(params.RouteToCluster, certOptions...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	req, err := c.prepareUserCertsRequest(params, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := c.authClient.GenerateUserCerts(ctx, *req)
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

// prepareUserCertsRequest creates a [proto.UserCertsRequest] with the fields
// set accordingly from the provided ReissueParams.
func (c *ClusterClient) prepareUserCertsRequest(params ReissueParams, key *Key) (*proto.UserCertsRequest, error) {
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

	attestationStatement, err := keys.GetAttestationStatement(key.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
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
		Format:                c.tc.CertificateFormat,
		RequesterName:         params.RequesterName,
		SSHLogin:              c.tc.HostLogin,
		AttestationStatement:  attestationStatement.ToProto(),
	}, nil
}

// performMFACeremony runs the mfa ceremony to completion. If successful the returned
// [Key] will be authorized to connect to the target.
func (c *ClusterClient) performMFACeremony(ctx context.Context, clt *ClusterClient, params ReissueParams, key *Key) (*Key, error) {
	stream, err := clt.authClient.GenerateUserSingleUseCerts(ctx)
	if err != nil {
		if trace.IsNotImplemented(err) {
			// Probably talking to an older server, use the old non-MFA endpoint.
			log.WithError(err).Debug("Auth server does not implement GenerateUserSingleUseCerts.")
			// SSH certs can be used without reissuing.
			if params.usage() == proto.UserCertsRequest_SSH && key.Cert != nil {
				return key, nil
			}

			key, err := clt.reissueUserCerts(ctx, CertCacheKeep, params)
			return key, trace.Wrap(err)
		}
		return nil, trace.Wrap(err)
	}
	defer func() {
		// CloseSend closes the client side of the stream
		stream.CloseSend()
		// Recv to wait for the server side of the stream to end, this needs to
		// be called to ensure the spans are finished properly
		stream.Recv()
	}()

	initReq, err := clt.prepareUserCertsRequest(params, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = stream.Send(&proto.UserSingleUseCertsRequest{Request: &proto.UserSingleUseCertsRequest_Init{
		Init: initReq,
	}})
	if err != nil {
		return nil, trace.Wrap(trail.FromGRPC(err))
	}

	resp, err := stream.Recv()
	if err != nil {
		err = trail.FromGRPC(err)
		// If connecting to a host in a leaf cluster and MFA failed check to see
		// if the leaf cluster requires MFA. If it doesn't return an error indicating
		// that MFA was not required instead of the error received from the root cluster.
		if c.cluster != clt.cluster {
			check, err := c.authClient.IsMFARequired(ctx, params.isMFARequiredRequest(clt.tc.HostLogin))
			if err != nil {
				return nil, trace.Wrap(MFARequiredUnknown(err))
			}
			if !check.Required {
				return nil, trace.Wrap(services.ErrSessionMFANotRequired)
			}
		}

		return nil, trace.Wrap(err)
	}
	mfaChal := resp.GetMFAChallenge()
	if mfaChal == nil {
		return nil, trace.BadParameter("server sent a %T on GenerateUserSingleUseCerts, expected MFAChallenge", resp.Response)
	}

	switch mfaChal.MFARequired {
	case proto.MFARequired_MFA_REQUIRED_NO:
		return nil, trace.Wrap(services.ErrSessionMFANotRequired)
	case proto.MFARequired_MFA_REQUIRED_UNSPECIFIED:
		// check if MFA is required with the auth client for this cluster and
		// not the root client
		check, err := c.authClient.IsMFARequired(ctx, params.isMFARequiredRequest(clt.tc.HostLogin))
		if err != nil {
			return nil, trace.Wrap(MFARequiredUnknown(err))
		}
		if !check.Required {
			return nil, trace.Wrap(services.ErrSessionMFANotRequired)
		}
	case proto.MFARequired_MFA_REQUIRED_YES:
		// Proceed with the prompt for MFA below.
	}

	mfaResp, err := clt.tc.PromptMFAChallenge(ctx, clt.tc.WebProxyAddr, mfaChal, nil /* applyOpts */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = stream.Send(&proto.UserSingleUseCertsRequest{Request: &proto.UserSingleUseCertsRequest_MFAResponse{MFAResponse: mfaResp}})
	if err != nil {
		return nil, trace.Wrap(trail.FromGRPC(err))
	}

	resp, err = stream.Recv()
	if err != nil {
		return nil, trace.Wrap(trail.FromGRPC(err))
	}
	certResp := resp.GetCert()
	if certResp == nil {
		return nil, trace.BadParameter("server sent a %T on GenerateUserSingleUseCerts, expected SingleUseUserCert", resp.Response)
	}
	switch crt := certResp.Cert.(type) {
	case *proto.SingleUseUserCert_SSH:
		key.Cert = crt.SSH
	case *proto.SingleUseUserCert_TLS:
		switch initReq.Usage {
		case proto.UserCertsRequest_Kubernetes:
			key.KubeTLSCerts[initReq.KubernetesCluster] = crt.TLS
		case proto.UserCertsRequest_Database:
			dbCert, err := makeDatabaseClientPEM(params.RouteToDatabase.Protocol, crt.TLS, key)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			key.DBTLSCerts[params.RouteToDatabase.ServiceName] = dbCert
		case proto.UserCertsRequest_WindowsDesktop:
			key.WindowsDesktopCerts[params.RouteToWindowsDesktop.WindowsDesktop] = crt.TLS
		default:
			return nil, trace.BadParameter("server returned a TLS certificate but cert request usage was %s", initReq.Usage)
		}
	default:
		return nil, trace.BadParameter("server sent a %T SingleUseUserCert in response", certResp.Cert)
	}
	key.ClusterName = params.RouteToCluster

	return key, nil
}

// CreateAccessRequest registers a new access request with the auth server.
func (c *ClusterClient) CreateAccessRequest(ctx context.Context, req types.AccessRequest) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/CreateAccessRequest",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(attribute.String("request", req.GetName())),
	)
	defer span.End()

	return trace.Wrap(c.authClient.CreateAccessRequest(ctx, req))
}

// GetAccessRequests loads all access requests matching the supplied filter.
func (c *ClusterClient) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/GetAccessRequests",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("id", filter.ID),
			attribute.String("user", filter.User),
		),
	)
	defer span.End()

	reqs, err := c.authClient.GetAccessRequests(ctx, filter)
	return reqs, trace.Wrap(err)
}

// GetRole loads a role resource by name.
func (c *ClusterClient) GetRole(ctx context.Context, name string) (types.Role, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/GetRole",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("role", name),
		),
	)
	defer span.End()

	role, err := c.authClient.GetRole(ctx, name)
	return role, trace.Wrap(err)
}

// NewWatcher sets up a new event watcher.
func (c *ClusterClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/NewWatcher",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("name", watch.Name),
		),
	)
	defer span.End()

	watcher, err := c.authClient.NewWatcher(ctx, watch)
	return watcher, trace.Wrap(err)
}

// GetClusterAlerts returns a list of matching alerts from the current cluster.
func (c *ClusterClient) GetClusterAlerts(ctx context.Context, req types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/GetClusterAlerts",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	alerts, err := c.authClient.GetClusterAlerts(ctx, req)
	return alerts, trace.Wrap(err)
}

// FindNodesByFiltersForCluster returns list of the nodes in a specified cluster which have filters matched.
func (c *ClusterClient) FindNodesByFiltersForCluster(ctx context.Context, req *proto.ListResourcesRequest, cluster string) ([]types.Server, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/FindNodesByFiltersForCluster",
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

	authClient, err := c.ConnectToCluster(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer authClient.Close()

	servers, err := client.GetAllResources[types.Server](ctx, authClient, req)
	return servers, trace.Wrap(err)
}

// FindAppServersByFilters returns a list of application servers in the current cluster which have filters matched.
func (c *ClusterClient) FindAppServersByFilters(ctx context.Context, req proto.ListResourcesRequest) ([]types.AppServer, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/FindAppServersByFilters",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("resource", req.ResourceType),
			attribute.Int("limit", int(req.Limit)),
			attribute.String("predicate", req.PredicateExpression),
			attribute.StringSlice("keywords", req.SearchKeywords),
		),
	)
	defer span.End()

	servers, err := c.FindAppServersByFiltersForCluster(ctx, req, c.cluster)
	return servers, trace.Wrap(err)
}

// FindAppServersByFiltersForCluster returns a list of application servers for a given cluster which have filters matched.
func (c *ClusterClient) FindAppServersByFiltersForCluster(ctx context.Context, req proto.ListResourcesRequest, cluster string) ([]types.AppServer, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/FindAppServersByFiltersForCluster",
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

	authClient, err := c.ConnectToCluster(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer authClient.Close()

	servers, err := client.GetAllResources[types.AppServer](ctx, authClient, &req)
	return servers, trace.Wrap(err)
}

// CreateAppSession creates a new application access session.
func (c *ClusterClient) CreateAppSession(ctx context.Context, req types.CreateAppSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/CreateAppSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("username", req.Username),
			attribute.String("cluster", req.ClusterName),
		),
	)
	defer span.End()

	authClient, err := c.RootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer authClient.Close()

	ws, err := authClient.CreateAppSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Make sure to wait for the created app session to propagate through the cache.
	clientConfig := c.ClientConfig(ctx, c.root)
	accessPoint, err := auth.NewClient(clientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer accessPoint.Close()

	err = auth.WaitForAppSession(ctx, ws.GetName(), ws.GetUser(), accessPoint)
	return ws, trace.Wrap(err)
}

// GetAppSession creates a new application access session.
func (c *ClusterClient) GetAppSession(ctx context.Context, req types.GetAppSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/GetAppSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	authClient, err := c.RootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ws, err := authClient.GetAppSession(ctx, req)
	return ws, trace.Wrap(err)
}

// DeleteAppSession removes the specified application access session.
func (c *ClusterClient) DeleteAppSession(ctx context.Context, sessionID string) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/DeleteAppSession",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("session", sessionID),
		),
	)
	defer span.End()

	authClient, err := c.RootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer authClient.Close()

	err = authClient.DeleteAppSession(ctx, types.DeleteAppSessionRequest{SessionID: sessionID})
	return trace.Wrap(err)
}
