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

package vnet

import (
	"context"
	"crypto"
	"crypto/rsa"
	"io"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// clientApplicationServiceClient is a gRPC client for the client application
// service. This client is used in the VNet admin process to make requests to
// the VNet client application.
type clientApplicationServiceClient struct {
	clt  vnetv1.ClientApplicationServiceClient
	conn *grpc.ClientConn
}

func newClientApplicationServiceClient(ctx context.Context, creds *credentials, addr string) (*clientApplicationServiceClient, error) {
	tlsConfig, err := creds.clientTLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(grpccredentials.NewTLS(tlsConfig)),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating user process gRPC client")
	}
	return &clientApplicationServiceClient{
		clt:  vnetv1.NewClientApplicationServiceClient(conn),
		conn: conn,
	}, nil
}

func (c *clientApplicationServiceClient) close() error {
	return trace.Wrap(c.conn.Close())
}

// Authenticate process authenticates the client application process.
func (c *clientApplicationServiceClient) AuthenticateProcess(ctx context.Context, pipePath string) error {
	resp, err := c.clt.AuthenticateProcess(ctx, &vnetv1.AuthenticateProcessRequest{
		Version:  api.Version,
		PipePath: pipePath,
	})
	if err != nil {
		return trace.Wrap(err, "calling AuthenticateProcess rpc")
	}
	if resp.Version != api.Version {
		return trace.BadParameter("version mismatch, user process version is %s, admin process version is %s",
			resp.Version, api.Version)
	}
	return nil
}

func (c *clientApplicationServiceClient) ReportNetworkStackInfo(ctx context.Context, nsi *vnetv1.NetworkStackInfo) error {
	if _, err := c.clt.ReportNetworkStackInfo(ctx, &vnetv1.ReportNetworkStackInfoRequest{
		NetworkStackInfo: nsi,
	}); err != nil {
		return trace.Wrap(err, "calling ReportNetworkStackInfo rpc")
	}
	return nil
}

// Ping pings the client application.
func (c *clientApplicationServiceClient) Ping(ctx context.Context) error {
	if _, err := c.clt.Ping(ctx, &vnetv1.PingRequest{}); err != nil {
		return trace.Wrap(err, "calling Ping rpc")
	}
	return nil
}

// ResolveFQDN resolves a query for a fully-qualified domain name to a target.
func (c *clientApplicationServiceClient) ResolveFQDN(ctx context.Context, fqdn string) (*vnetv1.ResolveFQDNResponse, error) {
	resp, err := c.clt.ResolveFQDN(ctx, &vnetv1.ResolveFQDNRequest{
		Fqdn: fqdn,
	})
	// Convert NotFound errors to errNoTCPHandler, which is what the network
	// stack is looking for.
	if trace.IsNotFound(err) {
		return nil, errNoTCPHandler
	}
	if err != nil {
		return nil, trace.Wrap(err, "calling ResolveFQDN rpc")
	}
	return resp, nil
}

// ReissueAppCert issues a new certificate for the requested app.
func (c *clientApplicationServiceClient) ReissueAppCert(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) ([]byte, error) {
	resp, err := c.clt.ReissueAppCert(ctx, &vnetv1.ReissueAppCertRequest{
		AppInfo:    appInfo,
		TargetPort: uint32(targetPort),
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling ReissueAppCert rpc")
	}
	return resp.GetCert(), nil
}

// SignForApp returns a cryptographic signature with the key associated with the
// requested app. The key resides in the client application process.
func (c *clientApplicationServiceClient) SignForApp(ctx context.Context, req *vnetv1.SignForAppRequest) ([]byte, error) {
	resp, err := c.clt.SignForApp(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "calling SignForApp rpc")
	}
	return resp.GetSignature(), nil
}

// OnNewConnection reports a new TCP connection to the target app.
func (c *clientApplicationServiceClient) OnNewConnection(ctx context.Context, appKey *vnetv1.AppKey) error {
	_, err := c.clt.OnNewConnection(ctx, &vnetv1.OnNewConnectionRequest{
		AppKey: appKey,
	})
	if err != nil {
		return trace.Wrap(err, "calling OnNewConnection rpc")
	}
	return nil
}

// OnInvalidLocalPort reports a failed connection to an invalid local port for
// the target app.
func (c *clientApplicationServiceClient) OnInvalidLocalPort(ctx context.Context, appInfo *vnetv1.AppInfo, targetPort uint16) error {
	_, err := c.clt.OnInvalidLocalPort(ctx, &vnetv1.OnInvalidLocalPortRequest{
		AppInfo:    appInfo,
		TargetPort: uint32(targetPort),
	})
	if err != nil {
		return trace.Wrap(err, "calling OnInvalidLocalPort rpc")
	}
	return nil
}

// GetTargetOSConfiguration returns the configuration values that should be
// configured in the OS, including DNS zones that should be handled by the VNet
// DNS nameserver and the IPv4 CIDR ranges that should be routed to the VNet TUN
// interface.
func (c *clientApplicationServiceClient) GetTargetOSConfiguration(ctx context.Context) (*vnetv1.TargetOSConfiguration, error) {
	resp, err := c.clt.GetTargetOSConfiguration(ctx, &vnetv1.GetTargetOSConfigurationRequest{})
	if err != nil {
		return nil, trace.Wrap(err, "calling GetTargetOSConfiguration rpc")
	}
	return resp.GetTargetOsConfiguration(), nil
}

// UserTLSCert returns the user TLS certificate for the given profile.
func (c *clientApplicationServiceClient) UserTLSCert(ctx context.Context, profileName string) (*vnetv1.UserTLSCertResponse, error) {
	resp, err := c.clt.UserTLSCert(ctx, &vnetv1.UserTLSCertRequest{
		Profile: profileName,
	})
	return resp, trace.Wrap(err, "calling UserTLSCert rpc")
}

// SignForUserTLS returns a cryptographic signature with the key associated with
// the user TLS key for the requested profile.
func (c *clientApplicationServiceClient) SignForUserTLS(ctx context.Context, req *vnetv1.SignForUserTLSRequest) ([]byte, error) {
	resp, err := c.clt.SignForUserTLS(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err, "calling SignForUserTLS rpc")
	}
	return resp.GetSignature(), nil
}

// SessionSSHConfig returns user SSH configuration values for an SSH session.
func (c *clientApplicationServiceClient) SessionSSHConfig(ctx context.Context, target dialTarget, user string) (*vnetv1.SessionSSHConfigResponse, error) {
	resp, err := c.clt.SessionSSHConfig(ctx, &vnetv1.SessionSSHConfigRequest{
		Profile:     target.profile,
		RootCluster: target.rootCluster,
		LeafCluster: target.leafCluster,
		Address:     target.addr,
		User:        user,
	})
	return resp, trace.Wrap(err, "calling SessionSSHConfig rpc")
}

// SignForSSHSession signs a digest with the SSH private key associated with the
// session from a previous call to SessionSSHConfig.
func (c *clientApplicationServiceClient) SignForSSHSession(ctx context.Context, sessionID string, sign *vnetv1.SignRequest) ([]byte, error) {
	resp, err := c.clt.SignForSSHSession(ctx, &vnetv1.SignForSSHSessionRequest{
		SessionId: sessionID,
		Sign:      sign,
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling SignForSSHSession rpc")
	}
	return resp.GetSignature(), nil
}

// ExchangeSSHKeys sends hostPublicKey to the client application so that it
// can write an OpenSSH-compatible configuration file. It returns the user
// public key that should be trusted for incoming connections from third-party
// SSH clients.
func (c *clientApplicationServiceClient) ExchangeSSHKeys(ctx context.Context, hostPublicKey ssh.PublicKey) (ssh.PublicKey, error) {
	resp, err := c.clt.ExchangeSSHKeys(ctx, &vnetv1.ExchangeSSHKeysRequest{
		HostPublicKey: hostPublicKey.Marshal(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "calling ExchangeSSHKeys rpc")
	}
	userPublicKey, err := ssh.ParsePublicKey(resp.GetUserPublicKey())
	if err != nil {
		return nil, trace.Wrap(err, "parsing trusted user public key")
	}
	return userPublicKey, nil
}

// rpcSigner implements [crypto.Signer] for signatures that are issued by the
// client application over gRPC.
type rpcSigner struct {
	pub         crypto.PublicKey
	sendRequest func(signReq *vnetv1.SignRequest) ([]byte, error)
}

// Public implements [crypto.Signer.Public] and returns the public key
// associated with the signer.
func (s *rpcSigner) Public() crypto.PublicKey {
	return s.pub
}

// Sign implements [crypto.Signer.Sign] and issues a signature over digest.
func (s *rpcSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	req := &vnetv1.SignRequest{
		Digest: digest,
	}
	switch opts.HashFunc() {
	case 0:
		req.Hash = vnetv1.Hash_HASH_NONE
	case crypto.SHA256:
		req.Hash = vnetv1.Hash_HASH_SHA256
	default:
		return nil, trace.BadParameter("unsupported signature hash func %v", opts.HashFunc())
	}
	if pssOpts, ok := opts.(*rsa.PSSOptions); ok {
		saltLen := int32(pssOpts.SaltLength)
		req.PssSaltLength = &saltLen
	}
	return s.sendRequest(req)
}
