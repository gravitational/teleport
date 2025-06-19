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
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	proxyclient "github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/utils/sshutils"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// sshProvider provides methods necessary for VNet SSH access.
type sshProvider struct {
	cfg sshProviderConfig
	// hostCASigner is the host CA key used internally in VNet to terminate
	// connections from clients, it is not a Teleport CA used by any cluster.
	hostCASigner         ssh.Signer
	trustedUserPublicKey ssh.PublicKey
}

type sshProviderConfig struct {
	clt   *clientApplicationServiceClient
	clock clockwork.Clock
	// overrideNodeDialer can be used in tests to dial SSH nodes with the real
	// TLS configuration but without setting up the proxy transport service.
	overrideNodeDialer func(
		ctx context.Context,
		target dialTarget,
		tlsConfig *tls.Config,
		dialOpts *vnetv1.DialOptions,
		agent *sshAgent,
	) (net.Conn, error)
}

func newSSHProvider(ctx context.Context, cfg sshProviderConfig) (*sshProvider, error) {
	hostCAKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hostCASigner, err := ssh.NewSignerFromSigner(hostCAKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedUserPublicKey, err := cfg.clt.ExchangeSSHKeys(ctx, hostCASigner.PublicKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &sshProvider{
		cfg:                  cfg,
		hostCASigner:         hostCASigner,
		trustedUserPublicKey: trustedUserPublicKey,
	}, nil
}

// dial dials the target SSH host.
func (p *sshProvider) dial(ctx context.Context, target dialTarget, agent *sshAgent) (net.Conn, error) {
	userTLSCertResp, err := p.cfg.clt.UserTLSCert(ctx, target.profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rawCert := userTLSCertResp.GetCert()
	dialOpts := userTLSCertResp.GetDialOptions()
	tlsConfig, err := p.userTLSConfig(ctx, target.profile, rawCert, dialOpts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if p.cfg.overrideNodeDialer != nil {
		conn, err := p.cfg.overrideNodeDialer(ctx, target, tlsConfig, dialOpts, agent)
		return conn, trace.Wrap(err)
	}
	return p.dialViaProxy(ctx, target, tlsConfig, dialOpts, agent)
}

// dialViaProxy dials the target SSH host via the proxy transport service.
func (p *sshProvider) dialViaProxy(
	ctx context.Context,
	target dialTarget,
	tlsConfig *tls.Config,
	dialOpts *vnetv1.DialOptions,
	agent *sshAgent,
) (net.Conn, error) {
	// TODO(nklaassen): consider reusing proxy clients, need to figure out when
	// it's necessary to make a new client e.g. if the user's TLS credentials
	// are replaced by a relogin. For now it's simpler to make a new client for
	// every SSH dial.
	pclt, err := proxyclient.NewClient(ctx, proxyclient.ClientConfig{
		ProxyAddress:            dialOpts.GetWebProxyAddr(),
		TLSConfigFunc:           func(cluster string) (*tls.Config, error) { return tlsConfig, nil },
		ALPNConnUpgradeRequired: dialOpts.GetAlpnConnUpgradeRequired(),
		InsecureSkipVerify:      dialOpts.GetInsecureSkipVerify(),
		// This empty SSH client config should never be used, we dial to the
		// proxy over TLS only.
		SSHConfig: &ssh.ClientConfig{},
	})
	if err != nil {
		return nil, trace.Wrap(err, "building proxy client")
	}
	// Forward an SSH agent in case proxy recording mode is enabled in the cluster.
	// At this point there is no SSH key for the user yet and the agent is
	// empty, the SSH key will be added to the agent in [sshProvider.sessionSSHConfig].
	// This forwarded agent will be used only to make the next SSH connection
	// to the target SSH node, it is not actually forwarded to the target node
	// and does not prevent the client from forwarding its own agent if
	// requested.
	conn, _, err := pclt.DialHost(ctx, target.addr, target.cluster, agent)
	if err != nil {
		pclt.Close()
		return nil, trace.Wrap(err, "dialing target via proxy")
	}
	// Make sure to close the proxy client, but not until we're done with the
	// target connection or else it would close the underlying gRPC stream.
	conn = newConnWithExtraCloser(conn, pclt.Close)
	return conn, nil
}

func (p *sshProvider) userTLSConfig(
	ctx context.Context,
	profile string,
	rawCert []byte,
	dialOpts *vnetv1.DialOptions,
) (*tls.Config, error) {
	parsedCert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		return nil, trace.Wrap(err, "parsing user TLS certificate")
	}
	signer := &rpcSigner{
		pub: parsedCert.PublicKey,
		sendRequest: func(req *vnetv1.SignRequest) ([]byte, error) {
			return p.cfg.clt.SignForUserTLS(ctx, &vnetv1.SignForUserTLSRequest{
				Profile: profile,
				Sign:    req,
			})
		},
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{rawCert},
		PrivateKey:  signer,
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(dialOpts.GetRootClusterCaCertPool()) {
		return nil, trace.Errorf("failed to parse root cluster CA cert pool")
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		RootCAs:            caPool,
		ServerName:         dialOpts.GetSni(),
		InsecureSkipVerify: dialOpts.GetInsecureSkipVerify(),
	}, nil
}

func (p *sshProvider) sessionSSHConfig(
	ctx context.Context,
	target dialTarget,
	user string,
	agent *sshAgent,
) (*ssh.ClientConfig, error) {
	// TODO(nklaassen): cache session SSH configs so we don't have to regenerate
	// every time.
	resp, err := p.cfg.clt.SessionSSHConfig(ctx, target, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshPub, err := ssh.ParsePublicKey(resp.GetCert())
	if err != nil {
		return nil, trace.Wrap(err, "parsing session SSH cert")
	}
	sshCert, ok := sshPub.(*ssh.Certificate)
	if !ok {
		return nil, trace.BadParameter("expected ssh.Certificate, got %T", sshCert)
	}
	cryptoPub, ok := sshCert.Key.(ssh.CryptoPublicKey)
	if !ok {
		return nil, trace.BadParameter("expected SSH key to implement CryptoPublicKey, got %T", sshCert.Key)
	}
	sessionID := resp.GetSessionId()
	signer := &rpcSigner{
		pub: cryptoPub.CryptoPublicKey(),
		sendRequest: func(req *vnetv1.SignRequest) ([]byte, error) {
			return p.cfg.clt.SignForSSHSession(ctx, sessionID, req)
		},
	}
	sshSigner, err := ssh.NewSignerFromSigner(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certSigner, err := ssh.NewCertSigner(sshCert, sshSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Add the session SSH key to the SSH agent in case proxy recording mode is
	// enabled. Adding it to the agent here before returning an
	// ssh.ClientConfig guarantees the key is added to the agent before the
	// agent could be used.
	if err := agent.setSessionKey(certSigner); err != nil {
		return nil, trace.Wrap(err)
	}
	hostKeyCallback, err := buildHostKeyCallback(resp.GetTrustedCas(), p.cfg.clock)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(certSigner)},
		User:            user,
		HostKeyCallback: hostKeyCallback,
	}, nil
}

func buildHostKeyCallback(trustedCAs [][]byte, clock clockwork.Clock) (ssh.HostKeyCallback, error) {
	var caKeys []ssh.PublicKey
	for _, trustedCA := range trustedCAs {
		caKey, err := ssh.ParsePublicKey(trustedCA)
		if err != nil {
			return nil, trace.Wrap(err, "parsing trusted CA key")
		}
		caKeys = append(caKeys, caKey)
	}
	hostKeyCallback, err := sshutils.NewHostKeyCallback(sshutils.HostKeyCallbackConfig{
		GetHostCheckers: func() ([]ssh.PublicKey, error) {
			return caKeys, nil
		},
		Clock: clock,
	})
	return hostKeyCallback, trace.Wrap(err, "building host key callback")
}

type dialTarget struct {
	fqdn        string
	profile     string
	rootCluster string
	leafCluster string
	cluster     string
	hostname    string
	addr        string
}

func computeDialTarget(matchedCluster *vnetv1.MatchedCluster, fqdn string) dialTarget {
	// matchedCluster.LeafCluster will be set if the host was in a leaf
	// cluster, else it will be unset and the target cluster is the root.
	targetCluster := cmp.Or(matchedCluster.GetLeafCluster(), matchedCluster.GetRootCluster())
	targetHost := strings.TrimSuffix(fqdn, "."+fullyQualify(targetCluster))
	return dialTarget{
		fqdn:        fqdn,
		profile:     matchedCluster.GetProfile(),
		rootCluster: matchedCluster.GetRootCluster(),
		leafCluster: matchedCluster.GetLeafCluster(),
		cluster:     targetCluster,
		hostname:    targetHost,
		addr:        targetHost + ":0",
	}
}

// connWithExtraCloser embeds a net.Conn and overrides the Close method to close
// an extra closer. Useful when the lifetime of a client providing the net.Conn
// must be tied to the lifetime of the Conn.
type connWithExtraCloser struct {
	net.Conn
	extraCloser func() error
}

func newConnWithExtraCloser(conn net.Conn, extraCloser func() error) *connWithExtraCloser {
	return &connWithExtraCloser{
		Conn:        conn,
		extraCloser: extraCloser,
	}
}

// Close closes the net.Conn and the extra closer.
func (c *connWithExtraCloser) Close() error {
	return trace.NewAggregate(c.Conn.Close(), c.extraCloser())
}
