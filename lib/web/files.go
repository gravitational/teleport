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
	"context"
	"net/http"
	"strings"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils/scp"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// fileTransferRequest describes HTTP file transfer request
type fileTransferRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port]).
	server string
	// Login is Linux username to connect as.
	login string
	// Namespace is node namespace.
	namespace string
	// Cluster is the name of the remote cluster to connect to.
	cluster string
	// Cluster is the name of the remote cluster to connect to.
	remoteFilePath string
}

func (h *Handler) transferFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	req := fileTransferRequest{
		login:          p.ByName("login"),
		namespace:      p.ByName("namespace"),
		server:         p.ByName("server"),
		remoteFilePath: p.ByName("filepath"),
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ft := fileTransfer{
		ctx:           ctx,
		authClient:    clt,
		proxyHostPort: h.ProxyHostPort(),
	}

	isUpload := r.Method == http.MethodPost
	if isUpload {
		err = ft.upload(req, r)
	} else {
		err = ft.download(req, w)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

type fileTransfer struct {
	// ctx is a web session context for the currently logged in user.
	ctx           *SessionContext
	authClient    auth.ClientI
	proxyHostPort string
}

func (f *fileTransfer) download(req fileTransferRequest, w http.ResponseWriter) error {
	remoteDest, err := f.resolveRemoteDest(req.remoteFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	cmd, err := scp.CreateHTTPDownload(scp.HTTPTransferRequest{
		RemoteLocation: remoteDest,
		HTTPResponse:   w,
		User:           f.ctx.GetUser(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := f.createClient(req)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.ExecuteSCP(context.TODO(), cmd)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (f *fileTransfer) upload(req fileTransferRequest, httpReq *http.Request) error {
	remoteDest, err := f.resolveRemoteDest(req.remoteFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	cmd, err := scp.CreateHTTPUpload(scp.HTTPTransferRequest{
		RemoteLocation: remoteDest,
		HTTPRequest:    httpReq,
		User:           f.ctx.GetUser(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := f.createClient(req)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.ExecuteSCP(context.TODO(), cmd)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (f *fileTransfer) createClient(req fileTransferRequest) (*client.TeleportClient, error) {
	if !services.IsValidNamespace(req.namespace) {
		return nil, trace.BadParameter("invalid namespace %q", req.namespace)
	}

	if req.login == "" {
		return nil, trace.BadParameter("missing login")
	}

	servers, err := f.authClient.GetNodes(req.namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostName, hostPort, err := resolveServerHostPort(req.server, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", req.server, err)
	}

	cfg, err := makeTeleportClientConfig(f.ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.HostLogin = req.login
	cfg.SiteName = req.cluster
	cfg.Namespace = req.namespace
	cfg.ProxyHostPort = f.proxyHostPort
	cfg.Host = hostName
	cfg.HostPort = hostPort

	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	return tc, nil
}

func (f *fileTransfer) resolveRemoteDest(filepath string) (string, error) {
	if strings.HasPrefix(filepath, "/absolute/") {
		remoteDest := strings.TrimPrefix(filepath, "/absolute/")
		return "/" + remoteDest, nil
	}

	if strings.HasPrefix(filepath, "/relative/") {
		remoteDest := strings.TrimPrefix(filepath, "/relative/")
		return "./" + remoteDest, nil
	}

	return "", trace.BadParameter("invalid remote file path: %q", filepath)
}
