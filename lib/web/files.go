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

package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

// fileTransferRequest describes HTTP file transfer request
type fileTransferRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port]).
	serverID string
	// Login is Linux username to connect as.
	login string
	// Cluster is the name of the remote cluster to connect to.
	cluster string
	// remoteLocation is file remote location
	remoteLocation string
	// filename is a file name
	filename string
	// mfaResponse is an optional parameter that contains an mfa response string used to issue single use certs
	mfaResponse string
	// fileTransferRequestID is used to find a FileTransferRequest on a session
	fileTransferRequestID string
	// moderatedSessonID is an ID of a moderated session that has completed a
	// file transfer request approval process
	moderatedSessionID string
}

func (h *Handler) transferFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
	query := r.URL.Query()
	req := fileTransferRequest{
		cluster:               site.GetName(),
		login:                 p.ByName("login"),
		serverID:              p.ByName("server"),
		remoteLocation:        query.Get("location"),
		filename:              query.Get("filename"),
		mfaResponse:           query.Get("mfaResponse"),
		fileTransferRequestID: query.Get("fileTransferRequestId"),
		moderatedSessionID:    query.Get("moderatedSessionId"),
	}

	// Check for old query parameter, uses the same data structure.
	// TODO(Joerger): DELETE IN v19.0.0
	if req.mfaResponse == "" {
		req.mfaResponse = query.Get("webauthn")
	}

	var mfaResponse *proto.MFAAuthenticateResponse
	if req.mfaResponse != "" {
		var err error
		if mfaResponse, err = client.ParseMFAChallengeResponse([]byte(req.mfaResponse)); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Send an error if only one of these params has been sent. Both should exist or not exist together
	if (req.fileTransferRequestID != "") != (req.moderatedSessionID != "") {
		return nil, trace.BadParameter("fileTransferRequestId and moderatedSessionId must both be included in the same request.")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ft := fileTransfer{
		sctx:          sctx,
		authClient:    clt,
		proxyHostPort: h.ProxyHostPort(),
	}

	mfaReq, err := clt.IsMFARequired(r.Context(), &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{
			Node: &proto.NodeLogin{
				Node:  p.ByName("server"),
				Login: p.ByName("login"),
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if mfaReq.Required && mfaResponse == nil {
		return nil, trace.AccessDenied("MFA required for file transfer")
	}

	var cfg *sftp.Config
	isUpload := r.Method == http.MethodPost
	if isUpload {
		cfg, err = sftp.CreateHTTPUploadConfig(sftp.HTTPTransferRequest{
			Src:         req.filename,
			Dst:         req.remoteLocation,
			HTTPRequest: r,
		})
	} else {
		cfg, err = sftp.CreateHTTPDownloadConfig(sftp.HTTPTransferRequest{
			Src:          req.remoteLocation,
			Dst:          req.filename,
			HTTPResponse: w,
		})
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tc, err := ft.createClient(req, r, h.cfg.PROXYSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.mfaResponse != "" {
		if err = ft.issueSingleUseCert(mfaResponse, r, tc); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	ctx := r.Context()
	if req.fileTransferRequestID != "" {
		ctx = context.WithValue(ctx, sftp.ModeratedSessionID, req.moderatedSessionID)
	}

	accessPoint, err := site.CachingAccessPoint()
	if err != nil {
		h.logger.DebugContext(r.Context(), "Unable to get auth access point", "error", err)
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	getAgent := func() (teleagent.Agent, error) {
		return teleagent.NopCloser(tc.LocalAgent()), nil
	}
	cert, err := sctx.GetSSHCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ident, err := sshca.DecodeIdentity(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer := agentless.SignerFromSSHIdentity(ident, h.auth.accessPoint, tc.SiteName, tc.Username)

	conn, err := h.cfg.Router.DialHost(
		ctx,
		&utils.NetAddr{Addr: r.RemoteAddr},
		&h.cfg.ProxyWebAddr,
		req.serverID,
		"0",
		tc.SiteName,
		accessChecker.CheckAccessToRemoteCluster,
		getAgent,
		signer,
	)
	if err != nil {
		if errors.Is(err, teleport.ErrNodeIsAmbiguous) {
			const message = "error: ambiguous host could match multiple nodes\n\nHint: try addressing the node by unique id (ex: user@node-id)\n"
			return nil, trace.NotFound("%s", message)
		}

		return nil, trace.Wrap(err)
	}

	dialTimeout := defaults.DefaultIOTimeout
	if netConfig, err := accessPoint.GetClusterNetworkingConfig(ctx); err != nil {
		h.logger.DebugContext(r.Context(), "Unable to fetch cluster networking config", "error", err)
	} else {
		dialTimeout = netConfig.GetSSHDialTimeout()
	}

	sshConfig := &ssh.ClientConfig{
		User:            tc.HostLogin,
		Auth:            tc.AuthMethods,
		HostKeyCallback: tc.HostKeyCallback,
		Timeout:         dialTimeout,
	}

	nodeClient, err := client.NewNodeClient(
		ctx,
		sshConfig,
		conn,
		req.serverID+":0",
		req.serverID,
		tc,
		modules.GetModules().IsBoringBinary(),
	)
	if err != nil {
		// The close error is ignored instead of using [trace.NewAggregate] because
		// aggregate errors do not allow error inspection with things like [trace.IsAccessDenied].
		_ = conn.Close()

		return nil, trace.Wrap(err)
	}

	defer nodeClient.Close()

	if err := nodeClient.TransferFiles(ctx, cfg); err != nil {
		if errors.As(err, new(*sftp.NonRecursiveDirectoryTransferError)) {
			return nil, trace.Errorf("transferring directories through the Web UI is not supported at the moment, please use tsh scp -r")
		}

		return nil, trace.Wrap(err)
	}

	// We must return nil so that we don't write anything to
	// the response, which would corrupt the downloaded file.
	return nil, nil
}

type fileTransfer struct {
	// sctx is a web session context for the currently logged in user.
	sctx          *SessionContext
	authClient    authclient.ClientI
	proxyHostPort string
}

func (f *fileTransfer) createClient(req fileTransferRequest, httpReq *http.Request, proxySigner multiplexer.PROXYHeaderSigner) (*client.TeleportClient, error) {
	if req.login == "" {
		return nil, trace.BadParameter("missing login")
	}

	servers, err := f.authClient.GetNodes(httpReq.Context(), defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostName, hostPort, err := resolveServerHostPort(req.serverID, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", req.serverID, err)
	}

	cfg, err := makeTeleportClientConfig(httpReq.Context(), f.sctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.HostLogin = req.login
	cfg.SiteName = req.cluster
	if err := cfg.ParseProxyHost(f.proxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	cfg.Host = hostName
	cfg.HostPort = hostPort
	cfg.ClientAddr = httpReq.RemoteAddr
	cfg.PROXYSigner = proxySigner

	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	return tc, nil
}

// issueSingleUseCert will take an assertion response sent from a solved challenge in the web UI
// and use that to generate a cert. This cert is added to the Teleport Client as an authmethod that
// can be used to connect to a node.
func (f *fileTransfer) issueSingleUseCert(mfaResponse *proto.MFAAuthenticateResponse, httpReq *http.Request, tc *client.TeleportClient) error {
	pk, err := keys.ParsePrivateKey(f.sctx.cfg.Session.GetSSHPriv())
	if err != nil {
		return trace.Wrap(err)
	}

	// Always acquire certs from the root cluster, that is where both the user and their devices are registered.
	cert, err := f.sctx.cfg.RootClient.GenerateUserCerts(httpReq.Context(), proto.UserCertsRequest{
		SSHPublicKey: pk.MarshalSSHPublicKey(),
		Username:     f.sctx.GetUser(),
		Expires:      time.Now().Add(time.Minute).UTC(),
		MFAResponse:  mfaResponse,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	sshCert, err := sshutils.ParseCertificate(cert.SSH)
	if err != nil {
		return trace.Wrap(err)
	}
	am, err := sshutils.AsAuthMethod(sshCert, pk.Signer)
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AuthMethods = []ssh.AuthMethod{am}
	return nil
}
