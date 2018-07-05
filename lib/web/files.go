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

package web

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils/scp"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/ssh"
)

// fileTransferRequest describes HTTP file transfer request
type fileTransferRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port]).
	Server string
	// Login is Linux username to connect as.
	Login string
	// Namespace is node namespace.
	Namespace string
	// Cluster is the name of the remote cluster to connect to.
	Cluster string
	// Cluster is the name of the remote cluster to connect to.
	RemoteFilePath string
}

// changePassword updates users password based on the old password
func (h *Handler) uploadFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	req := fileTransferRequest{
		Login:          p.ByName("login"),
		Namespace:      p.ByName("namespace"),
		Server:         p.ByName("node"),
		RemoteFilePath: p.ByName("filepath"),
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

	err = ft.upload(req, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	req := fileTransferRequest{
		Login:          p.ByName("login"),
		Namespace:      p.ByName("namespace"),
		Server:         p.ByName("node"),
		RemoteFilePath: p.ByName("filepath"),
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

	err = ft.download(req, w)
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
	remoteDest, err := f.resolveRemoteDest(req.RemoteFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := f.createClient(req)
	if err != nil {
		return trace.Wrap(err)
	}

	cmd, err := scp.CreateHTTPDownloadCommand(remoteDest, w, tc.Stdout)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.RunSCPCommand(context.TODO(), cmd)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (f *fileTransfer) upload(req fileTransferRequest, httpReq *http.Request) error {
	remoteDest, err := f.resolveRemoteDest(req.RemoteFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := f.createClient(req)
	if err != nil {
		return trace.Wrap(err)
	}

	cmd, err := scp.CreateHTTPUploadCommand(remoteDest, httpReq, tc.Stdout)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.RunSCPCommand(context.TODO(), cmd)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (f *fileTransfer) createClient(req fileTransferRequest) (*client.TeleportClient, error) {
	if !services.IsValidNamespace(req.Namespace) {
		return nil, trace.BadParameter("invalid namespace %q", req.Namespace)
	}

	if req.Login == "" {
		return nil, trace.BadParameter("login: missing login")
	}

	servers, err := f.authClient.GetNodes(req.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostName, hostPort, err := resolveServerHostPort(req.Server, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", req.Server, err)
	}

	agent, cert, err := f.ctx.GetAgent()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	tlsConfig, err := f.ctx.ClientTLSConfig()
	if err != nil {
		return nil, trace.BadParameter("failed to get client TLS config: %v", err)
	}

	clientConfig := &client.Config{
		HostLogin:     req.Login,
		SiteName:      req.Cluster,
		Namespace:     req.Namespace,
		Username:      f.ctx.user,
		ProxyHostPort: f.proxyHostPort,
		// TODO: replace os io streams with custom streams
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		Stdin:            os.Stdin,
		Host:             hostName,
		HostPort:         hostPort,
		SkipLocalAuth:    true,
		TLS:              tlsConfig,
		AuthMethods:      []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		DefaultPrincipal: cert.ValidPrincipals[0],
		HostKeyCallback:  func(string, net.Addr, ssh.PublicKey) error { return nil },
	}

	tc, err := client.NewClient(clientConfig)
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
