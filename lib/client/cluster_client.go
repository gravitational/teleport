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
	"context"
	"errors"
	"net"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/client/proto"
	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/resumption"
	"github.com/gravitational/teleport/lib/services"
)

// ClusterClient facilitates communicating with both the
// Auth and Proxy services of a cluster.
type ClusterClient struct {
	tc          *TeleportClient
	ProxyClient *proxyclient.Client
	AuthClient  authclient.ClientI
	Tracer      oteltrace.Tracer
	cluster     string
	root        string
}

// ClusterName returns the name of the cluster that the client
// is connected to.
func (c *ClusterClient) ClusterName() string {
	return c.cluster
}

// RootClusterName returns the name of the root cluster. If the
// client is connected directly to the root cluster, then this
// returns the same value as [ClusterClient.ClusterName]. If
// the client is connected to a leaf cluster, this returns the
// root cluster associated with the leaf.
func (c *ClusterClient) RootClusterName() string {
	return c.root
}

// CurrentCluster returns an authenticated auth server client for the local cluster.
// The returned auth server client does not need to be closed, it will be closed
// when the ClusterClient is closed.
func (c *ClusterClient) CurrentCluster() authclient.ClientI {
	// The auth.ClientI is wrapped in an sharedAuthClient to prevent callers from
	// being able to close the client. The auth.ClientI is only to be closed
	// when the ClusterClient is closed.
	return sharedAuthClient{ClientI: c.AuthClient}
}

// ConnectToRootCluster connects to the auth server of the root cluster
// via proxy. It returns connected and authenticated auth server client.
func (c *ClusterClient) ConnectToRootCluster(ctx context.Context) (authclient.ClientI, error) {
	root, err := c.ConnectToCluster(ctx, c.root)
	return root, trace.Wrap(err)
}

// ConnectToCluster connects to the auth server of the given cluster via proxy. It returns connected and authenticated auth server client
func (c *ClusterClient) ConnectToCluster(ctx context.Context, clusterName string) (authclient.ClientI, error) {
	if c.cluster == clusterName {
		return c.CurrentCluster(), nil
	}

	clientConfig, err := c.ProxyClient.ClientConfig(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authClient, err := authclient.NewClient(clientConfig)
	return authClient, trace.Wrap(err)
}

// Close terminates the connections to Auth and Proxy.
func (c *ClusterClient) Close() error {
	// close auth client first since it is tunneled through the proxy client
	return trace.NewAggregate(c.AuthClient.Close(), c.ProxyClient.Close())
}

// DialHostWithResumption is [proxyclient.DialHost] called on the underlying
// [*proxyclient.Client] of the ClusterClient, but with additional logic that
// attempts to resume the connection if it's supported by the remote server and
// if it's not been disabled in the TeleportClient (with a command-line flag,
// typically).
func (c *ClusterClient) DialHostWithResumption(ctx context.Context, target, cluster string, keyring agent.ExtendedAgent) (net.Conn, proxyclient.ClusterDetails, error) {
	conn, details, err := c.ProxyClient.DialHost(ctx, target, cluster, keyring)
	if err != nil {
		return nil, proxyclient.ClusterDetails{}, trace.Wrap(err)
	}

	if c.tc.DisableSSHResumption {
		return conn, details, nil
	}

	conn, err = resumption.WrapSSHClientConn(ctx, conn, func(ctx context.Context, hostID string) (net.Conn, error) {
		// if the connection is being resumed it means that we didn't need the
		// agent in the first place
		var noAgent agent.ExtendedAgent
		conn, _, err := c.ProxyClient.DialHost(ctx, hostID+":0", cluster, noAgent)
		return conn, err
	})
	if err != nil {
		return nil, proxyclient.ClusterDetails{}, trace.Wrap(err)
	}

	return conn, details, nil
}

// ceremonyFailedErr indicates that the mfa ceremony was attempted unsuccessfully.
type ceremonyFailedErr struct {
	err error
}

// Error returns the error string of the wrapped error if one exists.
func (c ceremonyFailedErr) Error() string {
	if c.err == nil {
		return ""
	}

	return c.err.Error()
}

// ReissueUserCerts generates a new set of certificates for the user.
func (c *ClusterClient) ReissueUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) error {
	ctx, span := c.Tracer.Start(
		ctx,
		"clusterClient/ReissueUserCerts",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", c.cluster),
		),
	)
	defer span.End()

	key, err := c.generateUserCerts(ctx, cachePolicy, params)
	if err != nil {
		return trace.Wrap(err)
	}

	if cachePolicy == CertCacheDrop {
		c.tc.localAgent.DeleteUserCerts("", WithAllCerts...)
	}

	// save the cert to the local storage (~/.tsh usually):
	return trace.Wrap(c.tc.localAgent.AddKey(key))
}

func (c *ClusterClient) generateUserCerts(ctx context.Context, cachePolicy CertCachePolicy, params ReissueParams) (*Key, error) {
	if params.RouteToCluster == "" {
		params.RouteToCluster = c.cluster
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

	rootClient, err := c.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rootClient.Close()

	certs, err := rootClient.GenerateUserCerts(ctx, *req)
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

	sshConfig := c.ProxyClient.SSHConfig(user)

	if target.MFACheck != nil && !target.MFACheck.Required {
		return sshConfig, nil
	}

	key, err := c.tc.localAgent.GetKey(target.Cluster, WithAllCerts...)
	if err != nil {
		return nil, trace.Wrap(MFARequiredUnknown(err))
	}

	// Always connect to root for getting new credentials, but attempt to reuse
	// the existing client if possible.
	rootClusterName, err := key.RootClusterName()
	if err != nil {
		return nil, trace.Wrap(MFARequiredUnknown(err))
	}

	mfaClt := c
	if target.Cluster != rootClusterName {
		cfg, err := c.ProxyClient.ClientConfig(ctx, rootClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		authClient, err := authclient.NewClient(cfg)
		if err != nil {
			return nil, trace.Wrap(MFARequiredUnknown(err))
		}

		mfaClt = &ClusterClient{
			tc:          c.tc,
			ProxyClient: c.ProxyClient,
			AuthClient:  authClient,
			Tracer:      c.Tracer,
			cluster:     rootClusterName,
			root:        rootClusterName,
		}
		// only close the new auth client and not the copied cluster client.
		defer authClient.Close()
	}

	log.Debug("Attempting to issue a single-use user certificate with an MFA check.")
	key, err = c.performSessionMFACeremony(ctx,
		mfaClt,
		ReissueParams{
			NodeName:       nodeName(TargetNode{Addr: target.Addr}),
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
		return nil, trace.Wrap(ceremonyFailedErr{err})
	}

	sshConfig.Auth = []ssh.AuthMethod{am}
	return sshConfig, nil
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
		params.AccessRequests = activeRequests
	}

	expires := tlsCert.NotAfter
	if params.TTL != 0 && time.Now().Add(params.TTL).Before(expires) {
		expires = time.Now().Add(params.TTL)
	}

	return &proto.UserCertsRequest{
		PublicKey:             key.MarshalSSHPublicKey(),
		Username:              tlsCert.Subject.CommonName,
		Expires:               expires,
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
		AttestationStatement:  key.PrivateKey.GetAttestationStatement().ToProto(),
	}, nil
}

// performSessionMFACeremony runs the mfa ceremony to completion.
// If successful the returned [Key] will be authorized to connect to the target.
func (c *ClusterClient) performSessionMFACeremony(ctx context.Context, rootClient *ClusterClient, params ReissueParams, key *Key) (*Key, error) {
	certsReq, err := rootClient.prepareUserCertsRequest(params, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mfaRequiredReq := params.isMFARequiredRequest(c.tc.HostLogin)

	var promptOpts []mfa.PromptOpt
	switch {
	case params.NodeName != "":
		promptOpts = append(promptOpts, mfa.WithPromptReasonSessionMFA("Node", params.NodeName))
	case params.KubernetesCluster != "":
		promptOpts = append(promptOpts, mfa.WithPromptReasonSessionMFA("Kubernetes cluster", params.KubernetesCluster))
	case params.RouteToDatabase.ServiceName != "":
		promptOpts = append(promptOpts, mfa.WithPromptReasonSessionMFA("Database", params.RouteToDatabase.ServiceName))
	case params.RouteToApp.Name != "":
		promptOpts = append(promptOpts, mfa.WithPromptReasonSessionMFA("Application", params.RouteToApp.Name))
	}

	key, _, err = PerformSessionMFACeremony(ctx, PerformSessionMFACeremonyParams{
		CurrentAuthClient: c.AuthClient,
		RootAuthClient:    rootClient.AuthClient,
		MFACeremony:       c.tc.NewMFACeremony(),
		MFAAgainstRoot:    c.cluster == rootClient.cluster,
		MFARequiredReq:    mfaRequiredReq,
		CertsReq:          certsReq,
		Key:               key,
	}, promptOpts...)
	return key, trace.Wrap(err)
}

// IssueUserCertsWithMFA generates a single-use certificate for the user. If MFA is required
// to access the resource the provided [mfa.Prompt] will be used to perform the MFA ceremony.
func (c *ClusterClient) IssueUserCertsWithMFA(ctx context.Context, params ReissueParams) (*Key, proto.MFARequired, error) {
	ctx, span := c.Tracer.Start(
		ctx,
		"ClusterClient/IssueUserCertsWithMFA",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", c.tc.SiteName),
		),
	)
	defer span.End()

	if params.RouteToCluster == "" {
		params.RouteToCluster = c.tc.SiteName
	}

	key := params.ExistingCreds
	if key == nil {
		var err error
		key, err = c.tc.localAgent.GetKey(params.RouteToCluster, WithAllCerts...)
		if err != nil {
			return nil, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, trace.Wrap(err)
		}
	}

	certClient := c
	var mfaRequired bool
	if params.MFACheck == nil {
		var err error
		authClient := params.AuthClient
		if authClient == nil {
			authClient, err = c.ConnectToCluster(ctx, params.RouteToCluster)
			if err != nil {
				return nil, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, trace.Wrap(err)
			}
		}

		resp, err := authClient.IsMFARequired(ctx, params.isMFARequiredRequest(c.tc.HostLogin))
		if err != nil {
			return nil, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, trace.Wrap(err)
		}
		mfaRequired = resp.Required

		// If connected to the root cluster, store the client so that it
		// can be reused below.
		if params.RouteToCluster == c.root {
			certClient = &ClusterClient{
				tc:          c.tc,
				ProxyClient: c.ProxyClient,
				AuthClient:  authClient,
				Tracer:      c.Tracer,
				cluster:     c.root,
				root:        c.root,
			}
		}

		// only close the new auth client and not the copied cluster client.
		defer authClient.Close()
	} else {
		mfaRequired = params.MFACheck.Required
	}

	// SSH certs can be used without embedding the node name.
	if !mfaRequired && params.usage() == proto.UserCertsRequest_SSH && key.Cert != nil {
		return key, proto.MFARequired_MFA_REQUIRED_NO, nil
	}

	// At this point, a connection to the root cluster is required to generate
	// an MFA verified certificate OR to issue certificates with the target
	// embedded in them for routing.
	if params.RouteToCluster != certClient.root {
		authClient, err := c.ConnectToRootCluster(ctx)
		if err != nil {
			return nil, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, trace.Wrap(err)
		}

		certClient = &ClusterClient{
			tc:          c.tc,
			ProxyClient: c.ProxyClient,
			AuthClient:  authClient,
			Tracer:      c.Tracer,
			cluster:     c.root,
			root:        c.root,
		}
		// only close the new auth client and not the copied cluster client.
		defer authClient.Close()
	}

	// MFA is not required, but the user requires a new certificate with the
	// target included in it for routing.
	if !mfaRequired {
		log.Debug("MFA not required for access.")
		key, err := certClient.generateUserCerts(ctx, CertCacheKeep, params)
		return key, proto.MFARequired_MFA_REQUIRED_NO, trace.Wrap(err)
	}

	// Perform the MFA ceremony and retrieve a new key.
	key, err := c.performSessionMFACeremony(ctx, certClient, params, key)
	if err != nil {
		return nil, proto.MFARequired_MFA_REQUIRED_YES, trace.Wrap(err)
	}

	log.Debug("Issued single-use user certificate after an MFA check.")
	return key, proto.MFARequired_MFA_REQUIRED_YES, nil
}

// PerformSessionMFARootClient is a subset of Auth methods required for MFA.
// Used by [PerformSessionMFACeremony].
type PerformSessionMFARootClient interface {
	CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error)
	GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)
}

// PerformSessionMFACurrentClient is a subset of Auth methods required for MFA.
// Used by [PerformSessionMFACeremony].
type PerformSessionMFACurrentClient interface {
	IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error)
}

// PerformSessionMFACeremonyParams are the input parameters for [PerformSessionMFACeremony].
type PerformSessionMFACeremonyParams struct {
	// CurrentAuthClient is the Auth client for the target cluster.
	// Unused if MFAAgainstRoot is true.
	CurrentAuthClient PerformSessionMFACurrentClient
	// RootAuthClient is the Auth client for the root cluster.
	// This is the client used to acquire the authn challenge and issue the user
	// certificates.
	RootAuthClient PerformSessionMFARootClient
	// MFACeremony handles the MFA ceremony.
	MFACeremony *mfa.Ceremony

	// MFAAgainstRoot tells whether to run the MFA required check against root or
	// current cluster.
	MFAAgainstRoot bool
	// MFARequiredReq is the request for the MFA verification check.
	MFARequiredReq *proto.IsMFARequiredRequest
	// CertsReq is the request for new certificates.
	CertsReq *proto.UserCertsRequest

	// Key is the client key to add the new certificates to.
	// Optional.
	Key *Key
}

// PerformSessionMFACeremony issues single-use certificates via GenerateUserCerts,
// following its recommended RPC flow.
//
// It is a lower-level, less opinionated variant of
// [ProxyClient.IssueUserCertsWithMFA].
//
// It does the following:
//
//  1. if !params.MFAAgainstRoot: Call CurrentAuthClient.IsMFARequired, abort if
//     MFA is not required.
//  2. Call RootAuthClient.CreateAuthenticateChallenge, abort if MFA is not
//     required
//  3. Call params.PromptMFA to solve the authn challenge
//  4. Call RootAuthClient.GenerateUserCerts
//
// Returns the modified params.Key and the GenerateUserCertsResponse, or an error.
func PerformSessionMFACeremony(ctx context.Context, params PerformSessionMFACeremonyParams, promptOpts ...mfa.PromptOpt) (*Key, *proto.Certs, error) {
	rootClient := params.RootAuthClient
	currentClient := params.CurrentAuthClient
	mfaRequiredReq := params.MFARequiredReq

	// If connecting to a host in a leaf cluster and MFA failed check to see
	// if the leaf cluster requires MFA. If it doesn't return an error indicating
	// that MFA was not required instead of the error received from the root cluster.
	if mfaRequiredReq != nil && !params.MFAAgainstRoot {
		mfaRequiredResp, err := currentClient.IsMFARequired(ctx, mfaRequiredReq)
		log.Debugf("MFA requirement acquired from leaf, MFARequired=%s", mfaRequiredResp.GetMFARequired())
		switch {
		case err != nil:
			return nil, nil, trace.Wrap(MFARequiredUnknown(err))
		case !mfaRequiredResp.Required:
			return nil, nil, trace.Wrap(services.ErrSessionMFANotRequired)
		}
		mfaRequiredReq = nil // Already checked, don't check again at root.
	}

	params.MFACeremony.CreateAuthenticateChallenge = rootClient.CreateAuthenticateChallenge
	mfaResp, err := params.MFACeremony.Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
			ContextUser: &proto.ContextUser{},
		},
		MFARequiredCheck: mfaRequiredReq,
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
	}, promptOpts...)
	if errors.Is(err, &mfa.ErrMFANotRequired) {
		return nil, nil, trace.Wrap(services.ErrSessionMFANotRequired)
	} else if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// If mfaResp is nil, the ceremony was a no-op (no devices registered).
	// TODO(Joerger): CreateAuthenticateChallenge, should return
	// this error directly instead of an empty challenge, without
	// regressing https://github.com/gravitational/teleport/issues/36482.
	if mfaResp == nil {
		return nil, nil, trace.Wrap(authclient.ErrNoMFADevices)
	}

	// Issue certificate.
	certsReq := params.CertsReq
	certsReq.MFAResponse = mfaResp
	certsReq.Purpose = proto.UserCertsRequest_CERT_PURPOSE_SINGLE_USE_CERTS
	log.Debug("Issuing single-use certificate from unary GenerateUserCerts")
	newCerts, err := rootClient.GenerateUserCerts(ctx, *certsReq)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	key := params.Key

	// Nothing more to do.
	if key == nil {
		return nil, newCerts, nil
	}

	switch {
	case len(newCerts.SSH) > 0:
		key.Cert = newCerts.SSH
	case len(newCerts.TLS) > 0:
		switch certsReq.Usage {
		case proto.UserCertsRequest_Kubernetes:
			if key.KubeTLSCerts == nil {
				key.KubeTLSCerts = make(map[string][]byte)
			}
			key.KubeTLSCerts[certsReq.KubernetesCluster] = newCerts.TLS

		case proto.UserCertsRequest_Database:
			dbCert, err := makeDatabaseClientPEM(certsReq.RouteToDatabase.Protocol, newCerts.TLS, key)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			if key.DBTLSCerts == nil {
				key.DBTLSCerts = make(map[string][]byte)
			}
			key.DBTLSCerts[certsReq.RouteToDatabase.ServiceName] = dbCert

		case proto.UserCertsRequest_WindowsDesktop:
			if key.WindowsDesktopCerts == nil {
				key.WindowsDesktopCerts = make(map[string][]byte)
			}
			key.WindowsDesktopCerts[certsReq.RouteToWindowsDesktop.WindowsDesktop] = newCerts.TLS

		case proto.UserCertsRequest_App:
			if key.AppTLSCerts == nil {
				key.AppTLSCerts = make(map[string][]byte)
			}
			key.AppTLSCerts[certsReq.RouteToApp.Name] = newCerts.TLS

		default:
			return nil, nil, trace.BadParameter("server returned a TLS certificate but cert request usage was %s", certsReq.Usage)
		}
	}
	key.ClusterName = certsReq.RouteToCluster

	return key, newCerts, nil
}
