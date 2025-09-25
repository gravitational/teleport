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

package relaypeer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/status"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	relaypeeringv1alpha "github.com/gravitational/teleport/gen/proto/go/teleport/relaypeering/v1alpha"
	"github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type ClientAccessPoint interface {
	GetRelayServer(ctx context.Context, name string) (*presencev1.RelayServer, error)
}

// ClientConfig contains parameters for [NewClient].
type ClientConfig struct {
	// HostID is the host ID of the local machine, to avoid peering with
	// ourselves.
	HostID string
	// ClusterName is the name of the Teleport cluster we belong to.
	ClusterName string
	// GroupName is the relay group we belong to, to avoid attempting to connect
	// to relays from a different group.
	GroupName string
	// AccessPoint is used to read information about other relays in the same
	// group.
	AccessPoint ClientAccessPoint

	Log *slog.Logger

	GetCertificate func() (*tls.Certificate, error)
	GetPool        func() (*x509.CertPool, error)
	Ciphersuites   []uint16
}

// NewClient creates a [Client] with a given configuration.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.HostID == "" {
		return nil, trace.BadParameter("missing HostID")
	}
	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("missing ClusterName")
	}
	if cfg.GroupName == "" {
		return nil, trace.BadParameter("missing GroupName")
	}
	if cfg.AccessPoint == nil {
		return nil, trace.BadParameter("missing AccessPoint")
	}
	if cfg.Log == nil {
		return nil, trace.BadParameter("missing Log")
	}
	if cfg.GetCertificate == nil {
		return nil, trace.BadParameter("missing GetCertificate")
	}
	if cfg.GetPool == nil {
		return nil, trace.BadParameter("missing GetPool")
	}

	return &Client{
		hostID:      cfg.HostID,
		clusterName: cfg.ClusterName,
		groupName:   cfg.GroupName,
		accessPoint: cfg.AccessPoint,

		log: cfg.Log,

		getCertificate: cfg.GetCertificate,
		getPool:        cfg.GetPool,
		ciphersuites:   cfg.Ciphersuites,
	}, nil
}

// Client is used to open tunnel connections to services that are connected with
// relay tunnels to the local relay group, but don't have a tunnel available
// locally, and thus require bouncing the connection to a different peer relay
// in the same relay group. It implements the client side of the relay peer
// dialing protocol, and connects to peer relays through the advertised peer
// address in the relay_server heartbeats.
type Client struct {
	hostID      string
	clusterName string
	groupName   string
	accessPoint ClientAccessPoint

	log *slog.Logger

	getCertificate func() (*tls.Certificate, error)
	getPool        func() (*x509.CertPool, error)
	ciphersuites   []uint16
}

var _ proxy.RelayPeerDialFunc = (*Client)(nil).Dial

// Dial will try to open a connection to a target host (in "<host id>.<cluster
// name>" format) with a given tunnel type, trying to dial through a list of
// peer relay IDs in random order. The list is assumed to be owned by Dial and
// will be shuffled in place. The returned connection will have the given source
// (remote) and destination (local) addresses.
func (c *Client) Dial(ctx context.Context, dialTarget string, tunnelType types.TunnelType, relayIDs []string, src, dst net.Addr) (net.Conn, error) {
	// it's assumed that all peer relays are equivalent
	for _, relayID := range utils.ShuffleVisit(relayIDs) {
		if relayID == c.hostID {
			continue
		}
		nc, err := c.dialRelay(ctx, dialTarget, tunnelType, relayID, src, dst)
		if err == nil {
			c.log.DebugContext(ctx, "Successfully dialed through peer relay", "relay_id", relayID)
			return nc, nil
		}
		c.log.DebugContext(ctx, "Failed to dial through peer relay", "relay_id", relayID, "error", err, "target", dialTarget)
	}
	return nil, trace.ConnectionProblem(nil, "unable to reach dial target through relay peering")
}

// dialRelay tries to open a connection to a target through a specific peer
// relay.
func (c *Client) dialRelay(ctx context.Context, dialTarget string, tunnelType types.TunnelType, relayID string, src net.Addr, dst net.Addr) (net.Conn, error) {
	relayServer, err := c.accessPoint.GetRelayServer(ctx, relayID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if relayServer.GetSpec().GetRelayGroup() != c.groupName {
		return nil, trace.BadParameter("peer relay server belongs to different relay group")
	}

	peerAddr := relayServer.GetSpec().GetPeerAddr()
	if peerAddr == "" {
		return nil, trace.BadParameter("no peer addr in peer relay server")
	}

	cert, err := c.getCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool, err := c.getPool()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nc, err := new(net.Dialer).DialContext(ctx, "tcp", peerAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverName, _, err := net.SplitHostPort(peerAddr)
	if err != nil {
		serverName = peerAddr
	}

	tlsConfig := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return cert, nil
		},

		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if cs.NegotiatedProtocol == "" {
				return trace.NotImplemented("relay peer protocol not supported")
			}

			opts := x509.VerifyOptions{
				DNSName: "",

				Roots:         pool,
				Intermediates: nil,

				KeyUsages: []x509.ExtKeyUsage{
					x509.ExtKeyUsageServerAuth,
				},
			}
			if len(cs.PeerCertificates) > 1 {
				opts.Intermediates = x509.NewCertPool()
				for _, cert := range cs.PeerCertificates[1:] {
					opts.Intermediates.AddCert(cert)
				}
			}
			if _, err := cs.PeerCertificates[0].Verify(opts); err != nil {
				return trace.Wrap(err)
			}

			id, err := tlsca.FromSubject(cs.PeerCertificates[0].Subject, cs.PeerCertificates[0].NotAfter)
			if err != nil {
				return trace.Wrap(err)
			}

			if !slices.Contains(id.Groups, string(types.RoleRelay)) &&
				!slices.Contains(id.SystemRoles, string(types.RoleRelay)) {
				return trace.BadParameter("dialed server is not a relay (roles %+q, system roles %+q)", id.Groups, id.SystemRoles)
			}

			if id.Username != relayID+"."+c.clusterName {
				return trace.BadParameter("dialed server is the wrong relay (expected %+q, got %+q)", relayID, id.Username)
			}

			return nil
		},

		NextProtos: []string{simpleALPN},
		ServerName: serverName,

		CipherSuites: c.ciphersuites,
		MinVersion:   tls.VersionTLS12,
	}
	tc := tls.Client(nc, tlsConfig)

	explode := make(chan struct{})
	defuse := context.AfterFunc(ctx, func() {
		defer close(explode)
		tc.SetDeadline(time.Unix(1, 0))
	})
	defer defuse()

	if err := writeProto(tc, &relaypeeringv1alpha.DialRequest{
		TargetHostId:   dialTarget,
		ConnectionType: string(tunnelType),
		Source:         addrToProto(src),
		Destination:    addrToProto(dst),
	}); err != nil {
		defuse()
		_ = tc.Close()
		return nil, trace.Wrap(err)
	}

	resp := new(relaypeeringv1alpha.DialResponse)
	if err := readProto(tc, resp); err != nil {
		defuse()
		_ = tc.Close()
		return nil, trace.Wrap(err)
	}

	if !defuse() {
		<-explode
	}
	tc.SetDeadline(time.Time{})

	if err := trail.FromGRPC(status.FromProto(resp.GetStatus()).Err()); err != nil {
		_ = tc.Close()
		return nil, trace.Wrap(err)
	}

	return tc, nil
}
