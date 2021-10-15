/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// LocalRegister is used to generate host keys when a node or proxy is running
// within the same process as the Auth Server and as such, does not need to
// use provisioning tokens.
func LocalRegister(id IdentityID, authServer *Server, additionalPrincipals, dnsNames []string, remoteAddr string) (*Identity, error) {
	priv, pub, err := authServer.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsPub, err := PrivateKeyToPublicKeyTLS(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If local registration is happening and no remote address was passed in
	// (which means no advertise IP was set), use localhost.
	if remoteAddr == "" {
		remoteAddr = defaults.Localhost
	}
	certs, err := authServer.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               id.HostUUID,
			NodeName:             id.NodeName,
			Role:                 id.Role,
			AdditionalPrincipals: additionalPrincipals,
			RemoteAddr:           remoteAddr,
			DNSNames:             dnsNames,
			NoCache:              true,
			PublicSSHKey:         pub,
			PublicTLSKey:         tlsPub,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := ReadIdentityFromKeyPair(priv, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return identity, nil
}

// RegisterParams specifies parameters
// for first time register operation with auth server
type RegisterParams struct {
	// Token is a secure token to join the cluster
	Token string
	// ID is identity ID
	ID IdentityID
	// Servers is a list of auth servers to dial
	Servers []utils.NetAddr
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// DNSNames is a list of DNS names to add to x509 certificate
	DNSNames []string
	// PrivateKey is a PEM encoded private key (not passed to auth servers)
	PrivateKey []byte
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
	// CipherSuites is a list of cipher suites to use for TLS client connection
	CipherSuites []uint16
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string
	// CAPath is the path to the CA file.
	CAPath string
	// GetHostCredentials is a client that can fetch host credentials.
	GetHostCredentials HostCredentials
	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification.
	// Defaults to real clock if unspecified
	Clock clockwork.Clock
	// EC2IdentityDocument is used for Simplified Node Joining to prove the
	// identity of a joining EC2 instance.
	EC2IdentityDocument []byte
	// JoinMethod is the joining method used for this register request.
	JoinMethod types.JoinMethod
}

func (r *RegisterParams) setDefaults() {
	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}
}

// CredGetter is an interface for a client that can be used to get host
// credentials. This interface is needed because lib/client can not be imported
// in lib/auth due to circular imports.
type HostCredentials func(context.Context, string, bool, types.RegisterUsingTokenRequest) (*proto.Certs, error)

// Register is used to generate host keys when a node or proxy are running on
// different hosts than the auth server. This method requires provisioning
// tokens to prove a valid auth server was used to issue the joining request
// as well as a method for the node to validate the auth server.
func Register(params RegisterParams) (*Identity, error) {
	params.setDefaults()
	// Read in the token. The token can either be passed in or come from a file
	// on disk.
	token, err := utils.ReadToken(params.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.WithField("auth-servers", params.Servers).Debugf("Registering node to the cluster.")

	type registerMethod struct {
		call func(token string, params RegisterParams) (*Identity, error)
		desc string
	}
	registerThroughAuth := registerMethod{registerThroughAuth, "with auth server"}
	registerThroughProxy := registerMethod{registerThroughProxy, "via proxy server"}
	registerUsingIAMMethod := registerMethod{registerUsingIAMMethod, "with IAM method"}

	// by default, try to register directly through auth first, then proxy
	registerMethods := []registerMethod{registerThroughAuth, registerThroughProxy}
	if params.JoinMethod == types.JoinMethodIAM {
		// if using IAM method, that is the only option
		log.Debugf("Registering with IAM method.")
		registerMethods = []registerMethod{registerUsingIAMMethod}
	} else if params.GetHostCredentials == nil {
		log.Debugf("Missing client, it is not possible to register through proxy.")
		registerMethods = []registerMethod{registerThroughAuth}
	} else if authServerIsProxy(params.Servers) {
		log.Debugf("The first specified auth server appears to be a proxy.")
		registerMethods = []registerMethod{registerThroughProxy, registerThroughAuth}
	}

	var collectedErrs []error
	for _, method := range registerMethods {
		log.Infof("Attempting registration %s.", method.desc)
		ident, err := method.call(token, params)
		if err != nil {
			collectedErrs = append(collectedErrs, err)
			log.WithError(err).Debugf("Registration %s failed.", method.desc)
			continue
		}
		log.Infof("Successfully registered %s.", method.desc)
		return ident, nil
	}
	return nil, trace.NewAggregate(collectedErrs...)
}

// authServerIsProxy returns true if the first specified auth server
// to register with appears to be a proxy.
func authServerIsProxy(servers []utils.NetAddr) bool {
	if len(servers) == 0 {
		return false
	}
	port := servers[0].Port(0)
	return port == defaults.HTTPListenPort || port == teleport.StandardHTTPSPort
}

// registerThroughProxy is used to register through the proxy server.
func registerThroughProxy(token string, params RegisterParams) (*Identity, error) {
	if len(params.Servers) == 0 {
		return nil, trace.BadParameter("no auth servers set")
	}

	certs, err := params.GetHostCredentials(context.Background(),
		params.Servers[0].String(),
		lib.IsInsecureDevMode(),
		types.RegisterUsingTokenRequest{
			Token:                token,
			HostID:               params.ID.HostUUID,
			NodeName:             params.ID.NodeName,
			Role:                 params.ID.Role,
			AdditionalPrincipals: params.AdditionalPrincipals,
			DNSNames:             params.DNSNames,
			PublicTLSKey:         params.PublicTLSKey,
			PublicSSHKey:         params.PublicSSHKey,
			EC2IdentityDocument:  params.EC2IdentityDocument,
		})
	if err != nil {
		return nil, trace.Unwrap(err)
	}

	return ReadIdentityFromKeyPair(params.PrivateKey, certs)
}

func validateCertificates(certs []*x509.Certificate, clock clockwork.Clock) error {
	for _, cert := range certs {
		if err := utils.VerifyCertificateExpiry(cert, clock); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

type unauthenticatedClientConfig struct {
	Addrs    []string
	CAPins   []string
	CAPath   string
	Insecure bool
	Clock    clockwork.Clock
}

// newUnauthenticatedClient creates a new client to the auth server when the
// client does not yet have any credentials.
func newUnauthenticatedClient(ctx context.Context, cfg *unauthenticatedClientConfig) (*Client, error) {
	var clusterName string
	var rawClusterCAs []byte
	var err error

	// first, try to read given CA path
	if cfg.CAPath != "" {
		rawClusterCAs, err = utils.ReadPath(cfg.CAPath)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	// If given a proxy address the client will ask the proxy to use TLS
	// routing to forward the connection to the auth server. Fetch the
	// cluster name and the auth server's tls certs here to configure the
	// connection.
	for _, proxyAddr := range cfg.Addrs {
		clusterCAResponse, err := webclient.GetClusterCA(ctx, proxyAddr, cfg.Insecure)
		if err == nil {
			clusterName = clusterCAResponse.ClusterName
			rawClusterCAs = append(rawClusterCAs, clusterCAResponse.ClusterCAs...)
			break
		}
		log.WithError(err).Debugf("Failed to retrieve cluster CA certs from proxy "+
			"endpoint, %q is probably not a proxy", proxyAddr)
	}

	tlsConfig := &tls.Config{
		Time: cfg.Clock.Now,
	}

	// parse CAs certs if we have any
	if len(rawClusterCAs) > 0 {
		certs, err := tlsca.ParseCertificatePEMs(rawClusterCAs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := validateCertificates(certs, cfg.Clock); err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConfig.RootCAs = x509.NewCertPool()
		for _, cert := range certs {
			tlsConfig.RootCAs.AddCert(cert)
		}
	} else {
		tlsConfig.InsecureSkipVerify = true
	}

	// create a client with certs from above, if any
	unauthenticatedClient, err := NewClient(client.Config{
		Addrs: cfg.Addrs,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		ALPNSNIAuthDialClusterName: clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !tlsConfig.InsecureSkipVerify {
		// successfully got CA certs above, auth server is verified, return this
		// client
		return unauthenticatedClient, nil
	}

	if tlsConfig.InsecureSkipVerify && len(cfg.CAPins) == 0 {
		// couldn't get certs from disk or proxy and no CA pins were given
		log.Warnf("Joining cluster without validating the identity of the Auth " +
			"Server. This may open you up to a Man-In-The-Middle (MITM) attack if an " +
			"attacker can gain privileged network access. To remedy this, use the CA pin " +
			"value provided when join token was generated to validate the identity of " +
			"the Auth Server.")
		return unauthenticatedClient, nil
	}

	// try to get certs from auth and compare with ca pins
	clusterCAResp, err := unauthenticatedClient.GetClusterCACert()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// done with this client, will create a new one which verifies the TLS cert
	unauthenticatedClient.Close()

	certs, err := tlsca.ParseCertificatePEMs(clusterCAResp.TLSCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateCertificates(certs, cfg.Clock); err != nil {
		return nil, trace.Wrap(err)
	}

	// check that fetched certs match the given CA pins
	if err := utils.CheckSPKI(cfg.CAPins, certs); err != nil {
		return nil, trace.Wrap(err)
	}

	// create a new client which will verify the auth server's TLS cert
	tlsConfig.RootCAs = x509.NewCertPool()
	for _, cert := range certs {
		tlsConfig.RootCAs.AddCert(cert)
	}
	tlsConfig.InsecureSkipVerify = false

	unauthenticatedClient, err = NewClient(client.Config{
		Addrs: cfg.Addrs,
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})

	return unauthenticatedClient, trace.Wrap(err)
}

// registerUsingIAMMethod is used to register using the IAM join method. It is
// able to register through a proxy or through the auth server directly.
func registerUsingIAMMethod(token string, params RegisterParams) (*Identity, error) {
	ctx := context.Background()

	// create a client to the auth server, if given a proxy address then TLS
	// routing will be used to transparently connect to the auth server
	authClient, err := newUnauthenticatedClient(ctx, &unauthenticatedClientConfig{
		Addrs:    utils.NetAddrsToStrings(params.Servers),
		CAPins:   params.CAPins,
		CAPath:   params.CAPath,
		Insecure: lib.IsInsecureDevMode(),
		Clock:    params.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// initiate the gRPC stream
	stream, err := authClient.RegisterUsingIAMMethod(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// the first value received from the stream will be a challenge string which
	// must be included in the signed sts:GetCallerIdentity request
	challenge, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create the signed sts:GetCallerIdentity request and include the challenge
	signedRequest, err := createSignedSTSIdentityRequest(challenge.Challenge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// send the RegisterUsingTokenRequest on the gRPC stream
	if err := stream.Send(&types.RegisterUsingTokenRequest{
		Token:                token,
		HostID:               params.ID.HostUUID,
		NodeName:             params.ID.NodeName,
		Role:                 params.ID.Role,
		AdditionalPrincipals: params.AdditionalPrincipals,
		DNSNames:             params.DNSNames,
		PublicTLSKey:         params.PublicTLSKey,
		PublicSSHKey:         params.PublicSSHKey,
		STSIdentityRequest:   signedRequest,
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// the second value received from the gRPC stream contains the new signed
	// host certs
	certs, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ReadIdentityFromKeyPair(params.PrivateKey, certs.Certs)
}

// registerThroughAuth is used to register through the auth server.
func registerThroughAuth(token string, params RegisterParams) (*Identity, error) {
	// Build a client to the Auth Server. If a CA pin is specified require the
	// Auth Server is validated. Otherwise attempt to use the CA file on disk
	// but if it's not available connect without validating the Auth Server CA.
	client, err := newUnauthenticatedClient(context.TODO(), &unauthenticatedClientConfig{
		Addrs:    utils.NetAddrsToStrings(params.Servers),
		CAPath:   params.CAPath,
		CAPins:   params.CAPins,
		Insecure: lib.IsInsecureDevMode(),
		Clock:    params.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the SSH and X509 certificates for a node.
	certs, err := client.RegisterUsingToken(types.RegisterUsingTokenRequest{
		Token:                token,
		HostID:               params.ID.HostUUID,
		NodeName:             params.ID.NodeName,
		Role:                 params.ID.Role,
		AdditionalPrincipals: params.AdditionalPrincipals,
		DNSNames:             params.DNSNames,
		PublicTLSKey:         params.PublicTLSKey,
		PublicSSHKey:         params.PublicSSHKey,
		EC2IdentityDocument:  params.EC2IdentityDocument,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ReadIdentityFromKeyPair(params.PrivateKey, certs)
}

// ReRegisterParams specifies parameters for re-registering
// in the cluster (rotating certificates for existing members)
type ReRegisterParams struct {
	// Client is an authenticated client using old credentials
	Client ClientI
	// ID is identity ID
	ID IdentityID
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// DNSNames is a list of DNS Names to add to the x509 client certificate
	DNSNames []string
	// PrivateKey is a PEM encoded private key (not passed to auth servers)
	PrivateKey []byte
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
	// Rotation is the rotation state of the certificate authority
	Rotation types.Rotation
}

// ReRegister renews the certificates and private keys based on the client's existing identity.
func ReRegister(params ReRegisterParams) (*Identity, error) {
	hostID, err := params.ID.HostID()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := params.Client.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               hostID,
			NodeName:             params.ID.NodeName,
			Role:                 params.ID.Role,
			AdditionalPrincipals: params.AdditionalPrincipals,
			DNSNames:             params.DNSNames,
			PublicTLSKey:         params.PublicTLSKey,
			PublicSSHKey:         params.PublicSSHKey,
			Rotation:             &params.Rotation,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ReadIdentityFromKeyPair(params.PrivateKey, certs)
}

// LegacyCerts is equivalent to proto.Certs, but uses
// JSON keys expected by old clients (7.x and earlier)
// DELETE in 9.0.0 (Joerger/zmb3)
type LegacyCerts struct {
	SSHCert    []byte   `json:"cert"`
	TLSCert    []byte   `json:"tls_cert"`
	TLSCACerts [][]byte `json:"tls_ca_certs"`
	SSHCACerts [][]byte `json:"ssh_ca_certs"`
}

// LegacyCertsFromProto converts proto.Certs to LegacyCerts.
// DELETE in 9.0.0 (Joerger/zmb3)
func LegacyCertsFromProto(c *proto.Certs) *LegacyCerts {
	return &LegacyCerts{
		SSHCert:    c.SSH,
		TLSCert:    c.TLS,
		SSHCACerts: c.SSHCACerts,
		TLSCACerts: c.TLSCACerts,
	}
}

// UnmarshalLegacyCerts unmarshals the a legacy certs response as proto.Certs.
// DELETE in 9.0.0 (Joerger/zmb3)
func UnmarshalLegacyCerts(bytes []byte) (*proto.Certs, error) {
	var lc LegacyCerts
	if err := json.Unmarshal(bytes, &lc); err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.Certs{
		SSH:        lc.SSHCert,
		TLS:        lc.TLSCert,
		TLSCACerts: lc.TLSCACerts,
		SSHCACerts: lc.SSHCACerts,
	}, nil
}
