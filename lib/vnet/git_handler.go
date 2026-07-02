// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

type gitHandler struct {
	cfg *gitHandlerConfig
	log *slog.Logger

	mu         sync.Mutex
	localProxy *alpnproxy.LocalProxy
}

type gitHandlerConfig struct {
	gitInfo     *vnetv1.GitServerInfo
	gitProvider *gitProvider
	clock       clockwork.Clock
	// alwaysTrustRootClusterCA can be set in tests so that TLS dials to the
	// proxy always trust the root cluster CA rather than the system cert pool,
	// even when ALPN conn upgrades are not required.
	alwaysTrustRootClusterCA bool
	parentCtx                context.Context
}

func newGitHandler(cfg *gitHandlerConfig) *gitHandler {
	return &gitHandler{
		cfg: cfg,
		log: log.With(
			teleport.ComponentKey, teleport.Component("vnet", "git-handler"),
			"profile", cfg.gitInfo.GetGitServerKey().GetProfile(),
			"leaf_cluster", cfg.gitInfo.GetGitServerKey().GetLeafCluster(),
			"git_server", cfg.gitInfo.GetGitServerKey().GetName()),
	}
}

func (h *gitHandler) getOrInitializeLocalProxy(ctx context.Context) (*alpnproxy.LocalProxy, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.localProxy != nil {
		return h.localProxy, nil
	}

	gitCertIssuer := &gitCertIssuer{
		gitProvider: h.cfg.gitProvider,
		gitInfo:     h.cfg.gitInfo,
	}
	certChecker := client.NewCertChecker(gitCertIssuer, h.cfg.clock)
	middleware := &localProxyMiddleware{
		certChecker: certChecker,
		onNewConnection: func(ctx context.Context) error {
			return h.cfg.gitProvider.OnNewGitConnection(ctx, h.cfg.gitInfo.GetGitServerKey())
		},
	}

	h.log.DebugContext(ctx, "Creating local proxy for git server")
	lp, err := newLocalProxy(localProxyConfig{
		dialOptions:              h.cfg.gitInfo.GetDialOptions(),
		protocols:                []alpncommon.Protocol{alpncommon.ProtocolHTTP},
		parentContext:            h.cfg.parentCtx,
		middleware:               middleware,
		clock:                    h.cfg.clock,
		alwaysTrustRootClusterCA: h.cfg.alwaysTrustRootClusterCA,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.localProxy = lp
	return lp, nil
}

func (h *gitHandler) handleTCPConnector(ctx context.Context, _ uint16, connector func() (net.Conn, error)) error {
	lp, err := h.getOrInitializeLocalProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(lp.HandleTCPConnector(ctx, connector), "handling TCP connector")
}

// gitCertIssuer implements [client.CertIssuer] for VNet git server access.
type gitCertIssuer struct {
	gitProvider *gitProvider
	gitInfo     *vnetv1.GitServerInfo
	group       singleflight.Group
}

func (i *gitCertIssuer) CheckCert(cert *x509.Certificate) error {
	return nil
}

func (i *gitCertIssuer) IssueCert(ctx context.Context) (tls.Certificate, error) {
	certVal, err, _ := i.group.Do("", func() (any, error) {
		return i.gitProvider.ReissueGitCert(ctx, i.gitInfo)
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cert, ok := certVal.(tls.Certificate)
	if !ok {
		return tls.Certificate{}, trace.BadParameter("singleflight returned unexpected type %T", certVal)
	}
	return cert, nil
}

// RouteToGit returns a *proto.RouteToGit populated from gitInfo.
func RouteToGit(gitInfo *vnetv1.GitServerInfo) *proto.RouteToGit {
	return &proto.RouteToGit{
		GitServerName: gitInfo.GetGitServerKey().GetName(),
	}
}
