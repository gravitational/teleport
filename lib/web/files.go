/*
Copyright 2018 Gravitational, Inc.

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

package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
)

// fileTransferRequest describes HTTP file transfer request
type fileTransferRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port]).
	serverID string
	// Login is Linux username to connect as.
	login string
	// Namespace is node namespace.
	namespace string
	// Cluster is the name of the remote cluster to connect to.
	cluster string
	// remoteLocation is file remote location
	remoteLocation string
	// filename is a file name
	filename string
	// webauthn is an optional parameter that contains a webauthn response string used to issue single use certs
	webauthn string
}

func (h *Handler) transferFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	query := r.URL.Query()
	req := fileTransferRequest{
		cluster:        p.ByName("site"),
		login:          p.ByName("login"),
		serverID:       p.ByName("server"),
		remoteLocation: query.Get("location"),
		filename:       query.Get("filename"),
		namespace:      defaults.Namespace,
		webauthn:       query.Get("webauthn"),
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

	if mfaReq.Required && query.Get("webauthn") == "" {
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

	tc, err := ft.createClient(req, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.webauthn != "" {
		err = ft.issueSingleUseCert(req.webauthn, r, tc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	err = tc.TransferFiles(r.Context(), req.login, req.serverID+":0", cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We must return nil so that we don't write anything to
	// the response, which would corrupt the downloaded file.
	return nil, nil
}

type fileTransfer struct {
	// sctx is a web session context for the currently logged in user.
	sctx          *SessionContext
	authClient    auth.ClientI
	proxyHostPort string
}

func (f *fileTransfer) createClient(req fileTransferRequest, httpReq *http.Request) (*client.TeleportClient, error) {
	if !types.IsValidNamespace(req.namespace) {
		return nil, trace.BadParameter("invalid namespace %q", req.namespace)
	}

	if req.login == "" {
		return nil, trace.BadParameter("missing login")
	}

	servers, err := f.authClient.GetNodes(httpReq.Context(), req.namespace)
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
	cfg.Namespace = req.namespace
	if err := cfg.ParseProxyHost(f.proxyHostPort); err != nil {
		return nil, trace.BadParameter("failed to parse proxy address: %v", err)
	}
	cfg.Host = hostName
	cfg.HostPort = hostPort
	cfg.ClientAddr = httpReq.RemoteAddr

	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	return tc, nil
}

type mfaRequest struct {
	// WebauthnResponse is the response from authenticators.
	WebauthnAssertionResponse *wanlib.CredentialAssertionResponse `json:"webauthnAssertionResponse"`
}

// issueSingleUseCert will take an assertion response sent from a solved challenge in the web UI
// and use that to generate a cert. This cert is added to the Teleport Client as an authmethod that
// can be used to connect to a node.
func (f *fileTransfer) issueSingleUseCert(webauthn string, httpReq *http.Request, tc *client.TeleportClient) error {
	var mfaReq mfaRequest
	err := json.Unmarshal([]byte(webauthn), &mfaReq)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := client.GenerateRSAKey()
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := f.authClient.GenerateUserCerts(httpReq.Context(), proto.UserCertsRequest{
		PublicKey: key.MarshalSSHPublicKey(),
		Username:  f.sctx.GetUser(),
		Expires:   time.Now().Add(time.Minute).UTC(),
		MFAResponse: &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wanlib.CredentialAssertionResponseToProto(mfaReq.WebauthnAssertionResponse),
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	key.Cert = cert.SSH

	am, err := key.AsAuthMethod()
	if err != nil {
		return trace.Wrap(err)
	}

	tc.AuthMethods = []ssh.AuthMethod{am}

	return nil
}
