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

package relay

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"

	apiconstants "github.com/gravitational/teleport/api/constants"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// SNISuffix is the suffix of the server name that signals that a connection is
// intended for Kubernetes Access through a relay. The format of the domain
// label in front of SNISuffix depends on the desired target for the connection.
// For now, the only supported class of targets is identified by a prefix of
// [SNIPrefixForKubeCluster] ("cluster-") to signify a connection for a given
// Kubernetes cluster by name.
const SNISuffix = ".kuberelay." + apiconstants.APIDomain

// SNIPrefixForKubeCluster is the prefix of the domain label used to identify a
// Kubernetes cluster as the target for a passively routed Relay connection.
const SNIPrefixForKubeCluster = "cluster-"

// When adding more target types (for example "session-") keep in mind that the
// entire reason for the hashing scheme in [SNILabelForKubeCluster] is that
// individual labels are limited to 63 characters (and the total FQDN is limited
// to 253 characters, but that's not relevant for us since Kube agents can only
// have one wildcard label in front of [SNISuffix]), so mind the total length of
// the data you need to encode.

type RelayTunnelDialFunc = func(ctx context.Context, hostID string, tunnelType apitypes.TunnelType, src, dst net.Addr) (net.Conn, error)
type RelayPeerDialFunc = func(ctx context.Context, hostID string, tunnelType apitypes.TunnelType, relayIDs []string, src, dst net.Addr) (net.Conn, error)

type GetKubeServersWithFilterFunc = func(ctx context.Context, filter func(readonly.KubeServer) bool) ([]apitypes.KubeServer, error)

// PassiveForwarderConfig contains the parameters for [NewPassiveForwarder].
type PassiveForwarderConfig struct {
	Log *slog.Logger

	ClusterName string
	GroupName   string

	// GetKubeServersWithFilter should return the Kube server resources known to
	// the cluster that pass the given filter function.
	GetKubeServersWithFilter GetKubeServersWithFilterFunc

	// LocalDial should dial the given target if it's available as a reverse
	// tunnel attached to the local instance.
	LocalDial RelayTunnelDialFunc
	// PeerDial should dial the given target through a known list of relay IDs.
	// It will only be called for targets that are advertising the same relay
	// group, but the function should check that any given relay in the list
	// belongs to the correct group before attempting to use it.
	PeerDial RelayPeerDialFunc
}

// NewPassiveForwarder returns a [PassiveForwarder] with the given config.
func NewPassiveForwarder(cfg PassiveForwarderConfig) (*PassiveForwarder, error) {
	if cfg.Log == nil {
		return nil, trace.BadParameter("missing Log")
	}
	if cfg.ClusterName == "" {
		return nil, trace.BadParameter("missing ClusterName")
	}
	if cfg.GroupName == "" {
		return nil, trace.BadParameter("missing GroupName")
	}
	if cfg.GetKubeServersWithFilter == nil {
		return nil, trace.BadParameter("missing GetKubeServersWithFilter")
	}
	if cfg.LocalDial == nil {
		return nil, trace.BadParameter("missing LocalDial")
	}
	if cfg.PeerDial == nil {
		return nil, trace.BadParameter("missing PeerDial")
	}

	resolvedNames, err := lru.New[[hashLen]byte, string](64)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This context is owned by PassiveForwarder and it's used for operations
	// that get synchronously canceled by Close.
	ctx, cancel := context.WithCancel(context.Background())

	return &PassiveForwarder{
		log: cfg.Log,

		clusterName: cfg.ClusterName,
		groupName:   cfg.GroupName,

		getKubeServersWithFilter: cfg.GetKubeServersWithFilter,

		localDial: cfg.LocalDial,
		peerDial:  cfg.PeerDial,

		resolvedNames: resolvedNames,

		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// PassiveForwarder implements the logic to route and forward connections to
// Kubernetes Service agents connected to a Relay.
type PassiveForwarder struct {
	log *slog.Logger

	clusterName string
	groupName   string

	getKubeServersWithFilter GetKubeServersWithFilterFunc

	localDial RelayTunnelDialFunc
	peerDial  RelayPeerDialFunc

	// resolvedNames contains (hash, name) pairs for which
	// hash == hashForTarget(clusterName, name); pairs are only _added_ to the
	// LRU if v is a kube cluster served by an agent connected to this relay
	// group, but that doesn't guarantee that the cluster is still being served
	// if the pair is looked up at a later time.
	resolvedNames *lru.Cache[[hashLen]byte, string]

	// ctx is used to signal that long running operations should unblock and
	// return, especially operations that have added to wg because Close should
	// wait until their end. It's owned by PassiveForwarder and should be
	// canceled by calling Close; it does not belong to a context tree and does
	// not outlive any call with a parent context. The intended operation of
	// this object is almost entirely based on I/O and explicit closing, so
	// there's no natural way to receive contexts associated with individual
	// operations triggered by callers of its methods.
	ctx    context.Context
	cancel context.CancelFunc

	mu sync.Mutex
	// wg synchronizes with ctx and mu: wg.Add must only be called while holding
	// mu if ctx.Err == nil, wg.Wait must only be called after cancel is called
	// while holding mu.
	wg sync.WaitGroup
}

// Close closes all ongoing connections and blocks until the associated
// resources are gone.
func (p *PassiveForwarder) Close() error {
	p.mu.Lock()
	// we cancel the context while holding the lock to signal that nothing
	// long-running should begin in the background, so nothing will add to wg
	// and we can wait on it
	p.cancel()
	p.mu.Unlock()

	p.wg.Wait()
	return nil
}

// Dispatch checks if the given SNI refers to a connection for a Kubernetes
// cluster through a Relay; if so, it returns true and the connection should be
// ignored by the caller, otherwise the connection is left untouched and the
// caller should handle it. The transcript contains data that was already sent
// by the client (i.e. a TLS ClientHello) and will be sent to the agent before
// forwarding more data from the connection. This method is intended to be used
// with [relaytransport.SNIDispatchTransportCredentials].
func (p *PassiveForwarder) Dispatch(serverName string, transcript *bytes.Buffer, rawConn net.Conn) (dispatched bool) {
	sniPrefix, ok := strings.CutSuffix(serverName, SNISuffix)
	if !ok {
		return false
	}

	p.mu.Lock()
	if p.ctx.Err() != nil {
		p.mu.Unlock()
		_ = rawConn.Close()
		// the forwarder was closed and Close has potentially already returned,
		// so we shouldn't spawn additional goroutines
		return true
	}
	p.wg.Add(1)
	go func() {
		p.wg.Done()
		p.forward(sniPrefix, transcript, rawConn)
	}()
	p.mu.Unlock()

	return true
}

func (p *PassiveForwarder) forward(sniPrefix string, transcript *bytes.Buffer, clientConn net.Conn) {
	ctx := p.ctx
	log := p.log.With(
		"client_addr", clientConn.RemoteAddr().String(),
	)
	// TODO(espadolini): there's no way to signal errors to the client, so the
	// best we can do is to cut the connection abruptly - unfortunately, that
	// will cause clients such as kubectl to treat requests as retriable, and
	// it's only after a lengthy timeout that an error will actually be reported
	// to the user. Then again, this same behavior also means that if the
	// connection failure is caused by a temporary failure due to missing
	// tunnels and the likes, the following retries have a chance of succeeding,
	// so this behavior is not all bad.
	defer clientConn.Close()

	sniPrefix = strings.ToLower(sniPrefix)
	encodedHash, ok := strings.CutPrefix(sniPrefix, SNIPrefixForKubeCluster)
	if !ok || len(encodedHash) != encodedHashLen {
		log.DebugContext(ctx,
			"Received unsupported SNI for kube relay forwarding",
			"sni_prefix", sniPrefix,
		)
		return
	}
	var desiredHash [hashLen]byte
	if _, err := base32hex.Decode(desiredHash[:], []byte(encodedHash)); err != nil {
		log.DebugContext(ctx,
			"Received malformed hash in SNI for kube cluster relay forwarding",
			"encoded_hash", encodedHash,
		)
		return
	}

	kubeClusterName, cached := p.resolvedNames.Get(desiredHash)
	kubeServers, err := p.getKubeServersWithFilter(
		ctx,
		func(ks readonly.KubeServer) bool {
			if ks.GetRelayGroup() != p.groupName {
				return false
			}

			if kubeClusterName != "" {
				return ks.GetCluster().GetName() == kubeClusterName
			}

			if desiredHash == hashForTarget(p.clusterName, ks.GetCluster().GetName()) {
				kubeClusterName = ks.GetCluster().GetName()
				if !cached {
					p.resolvedNames.Add(desiredHash, kubeClusterName)
				}
				return true
			}
			return false
		},
	)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.ErrorContext(ctx,
			"Failed to retrieve kube servers for routing",
			"encoded_hash", encodedHash,
			"error", err,
		)
		return
	}
	if len(kubeServers) < 1 {
		log.WarnContext(ctx,
			"No kube server found for the desired hash in the local relay group",
			"encoded_hash", encodedHash,
		)
		return
	}

	var agentConn net.Conn
	var agentHostID string
	for kubeServer := range healthcheck.OrderByTargetHealthStatus(kubeServers) {
		target := kubeServer.GetHostID() + "." + p.clusterName

		localTunnelConn, err := p.localDial(ctx,
			target, apitypes.KubeTunnel,
			clientConn.RemoteAddr(), clientConn.LocalAddr(),
		)
		if err == nil {
			agentConn = localTunnelConn
			agentHostID = kubeServer.GetHostID()
			break
		}

		if !trace.IsNotFound(err) {
			log.DebugContext(ctx,
				"Failed to dial target in local relay tunnel server, trying peer dialing",
				"agent_host_id", kubeServer.GetHostID(),
				"error", err,
			)
		}

		peerTunnelConn, err := p.peerDial(ctx,
			target, apitypes.KubeTunnel,
			slices.Clone(kubeServer.GetRelayIDs()),
			clientConn.RemoteAddr(), clientConn.LocalAddr(),
		)
		if err == nil {
			agentConn = peerTunnelConn
			agentHostID = kubeServer.GetHostID()
			break
		}

		log.DebugContext(ctx,
			"Failed to dial target through relay peering, trying next target if any",
			"agent_host_id", kubeServer.GetHostID(),
			"error", err,
		)
	}
	if agentConn == nil {
		if ctx.Err() != nil {
			return
		}
		log.WarnContext(ctx,
			"Failed to open tunnel connection to any agent connected to this relay group serving the desired cluster",
			"kube_cluster", kubeClusterName,
		)
		return
	}
	defer agentConn.Close()

	defer context.AfterFunc(ctx, func() {
		// this will unblock both sides of the copy (it's not sufficient to
		// close just one, it might've already half-closed and the other
		// direction might get stuck reading)
		_ = agentConn.Close()
		_ = clientConn.Close()
	})()

	log = log.With(
		"kube_cluster", kubeClusterName,
		"agent_host_id", agentHostID,
	)
	log.InfoContext(ctx,
		"Forwarding kube connection to agent",
	)
	defer log.DebugContext(ctx,
		"Done forwarding kube connection to agent",
	)

	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Log(ctx, logutils.TraceLevel,
			"Done copying data from client to agent",
		)
		if _, err := io.Copy(agentConn, transcript); err != nil {
			log.Log(ctx, traceIfOKNetworkError(err, slog.LevelDebug),
				"Got an error copying data from client transcript to agent",
				"error", err,
			)
			_ = clientConn.Close()
			_ = agentConn.Close()
			return
		}
		if _, err := io.Copy(agentConn, clientConn); err != nil {
			log.Log(ctx, traceIfOKNetworkError(err, slog.LevelDebug),
				"Got an error copying data from client to agent",
				"error", err,
			)
			_ = clientConn.Close()
			_ = agentConn.Close()
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer log.Log(ctx, logutils.TraceLevel,
			"Done copying data from agent to client",
		)
		if _, err := io.Copy(clientConn, agentConn); err != nil {
			log.Log(ctx, traceIfOKNetworkError(err, slog.LevelDebug),
				"Got an error copying data from agent to client",
				"error", err,
			)
			_ = clientConn.Close()
			_ = agentConn.Close()
			return
		}
	}()
}

func traceIfOKNetworkError(err error, l slog.Level) slog.Level {
	if err == nil || utils.IsOKNetworkError(err) {
		return logutils.TraceLevel
	}
	return l
}
