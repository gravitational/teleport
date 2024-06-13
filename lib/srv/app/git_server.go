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

package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// githubSSHAddress is the SSH address for github.com
	githubSSHAddress = "github.com:22"
)

type gitSession struct {
	app      types.Application
	identity *tlsca.Identity
	emitter  apievents.Emitter

	// command is either gittransport.UploadPackServiceName or gittransport.ReceivePackServiceName
	command string
	path    string
}

type gitServer struct {
	authClient authclient.ClientI

	emitter           apievents.Emitter
	serverConfigCache *utils.FnCache
	cache             *utils.FnCache
}

func newGitServer(authClient authclient.ClientI, emitter apievents.Emitter) (*gitServer, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL: time.Hour,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &gitServer{
		authClient: authClient,
		cache:      cache,
		emitter:    emitter,
	}, nil
}

func (s *gitServer) handleConnection(ctx context.Context, conn net.Conn, identity *tlsca.Identity, app types.Application) error {
	slog.DebugContext(ctx, "Handling incoming GitHub connection.", "app", app.GetName(), "user", identity.Username, "route", identity.RouteToApp)
	defer slog.DebugContext(ctx, "Completed incoming GitHub connection.", "app", app.GetName(), "user", identity.Username, identity.RouteToApp)

	sshServerConfig, err := s.getServerConfig(ctx, app)
	if err != nil {
		return trace.Wrap(err)
	}
	clientConn, clientChans, clientReqs, err := ssh.NewServerConn(conn, sshServerConfig)
	if err != nil {
		return trace.BadParameter("Failed to establish SSH connection: %v", err)
	}
	defer clientConn.Close()

	ghConn, ghChans, ghReqs, err := s.connectGitHub(ctx, identity)
	if err != nil {
		// TODO is it possible to send an error to client ?
		return trace.Wrap(err)
	}
	defer ghConn.Close()

	go copyGlobalRequests(ctx, ghConn, clientReqs, "client")
	go copyGlobalRequests(ctx, clientConn, ghReqs, "GitHub")
	go rejectChannels(ctx, ghChans, "GitHub")

	for {
		select {
		case newChannel := <-clientChans:
			if newChannel == nil {
				return nil
			}
			if err := s.handleChannel(ctx, identity, app, ghConn, newChannel); err != nil {
				return trace.Wrap(err)
			}

		case <-ctx.Done():
			return nil
		}
	}

	return nil
}

func (s *gitServer) getServerConfig(ctx context.Context, app types.Application) (*ssh.ServerConfig, error) {
	type serverCacheKey string
	return utils.FnCacheGet(ctx, s.cache, serverCacheKey(app.GetName()), func(ctx context.Context) (*ssh.ServerConfig, error) {
		slog.DebugContext(ctx, "Generating git server config.", "app", app.GetName())
		privBytes, pubBytes, err := native.GenerateKeyPair()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		response, err := s.authClient.GenerateGitServerCert(ctx, &proto.GenerateGitServerCertRequest{
			PublicKey: pubBytes,
			AppName:   app.GetName(),
			TTL:       proto.Duration(time.Hour),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// create a *ssh.Certificate
		privateKey, err := ssh.ParsePrivateKey(privBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert, err := apisshutils.ParseCertificate(response.SshCertificate)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.NewCertSigner(cert, privateKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sshServerConfig := &ssh.ServerConfig{
			NoClientAuth: true, // Accept any client without authentication
		}
		sshServerConfig.AddHostKey(signer)
		return sshServerConfig, nil
	})
}

func (s *gitServer) connectGitHub(ctx context.Context, identity *tlsca.Identity) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	userConfig, err := s.getGitHubUserConfig(ctx, identity)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	conn, err := net.Dial("tcp", githubSSHAddress)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, githubSSHAddress, userConfig)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Connected to GitHub")
	return c, chans, reqs, nil
}

func (s *gitServer) getGitHubLogin(identity *tlsca.Identity) (string, error) {
	// TODO access check
	return identity.RouteToApp.GitHubUsername, nil
}

func (s *gitServer) getGitHubUserConfig(ctx context.Context, identity *tlsca.Identity) (*ssh.ClientConfig, error) {
	login, err := s.getGitHubLogin(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	now := time.Now()
	ttl := identity.Expires.Sub(time.Now())
	if ttl > time.Hour {
		ttl = time.Hour
	}

	key := struct {
		Login string
		KeyID string
	}{
		Login: login,
		KeyID: identity.Username,
	}
	userConfig, err := utils.FnCacheGetWithTTL(ctx, s.cache, key, ttl, func(ctx context.Context) (*ssh.ClientConfig, error) {
		slog.DebugContext(ctx, "Generating user config for GitHub.", "identity", identity, "username", login)

		privateKeyBytes, publicKeyBytes, err := native.GenerateKeyPair()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response, err := s.authClient.SignGitHubUserCert(ctx, &proto.SignGitHubUserCertRequest{
			PublicKey: publicKeyBytes,
			Login:     key.Login,
			KeyID:     key.KeyID,
			// TODO change Expires filed to TTL for simplicity.
			Expires: now.Add(ttl),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		signer, err := sshutils.NewSigner(privateKeyBytes, response.AuthorizedKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &ssh.ClientConfig{
			User: "git",
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}, nil
	})
	return userConfig, trace.Wrap(err)
}

func (s *gitServer) handleChannel(ctx context.Context, identity *tlsca.Identity, app types.Application, ghConn ssh.Conn, clientChan ssh.NewChannel) error {
	slog.DebugContext(ctx, "Received Git channel.", "chan", clientChan.ChannelType())

	if clientChan.ChannelType() != "session" {
		clientChan.Reject(ssh.UnknownChannelType, "unsupported channel type")
		return nil
	}

	clientSessCh, clientSessReqs, err := clientChan.Accept()
	if err != nil {
		slog.DebugContext(ctx, "Failed to accept client channel.", "error", err)
		return trace.Wrap(err)
	}

	ghSessCh, ghSessReqs, err := ghConn.OpenChannel("session", nil)
	if err != nil {
		return trace.Wrap(err)
	}

	gitSession := &gitSession{
		app:      app,
		identity: identity,
	}

	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		defer wg.Done()
		auditor := &clientToServer{
			gitSession: gitSession,
		}
		defer auditor.tryAudit(ctx, s.emitter)
		defer ghSessCh.CloseWrite()
		n, err := io.Copy(io.MultiWriter(auditor, ghSessCh), clientSessCh)
		slog.DebugContext(ctx, "client -> GitHub", "error", err, "written", n)
	}()
	go func() {
		defer wg.Done()
		n, err := io.Copy(io.MultiWriter(&serverToClient{}, clientSessCh), ghSessCh)
		slog.DebugContext(ctx, "GitHub -> client", "error", err, "written", n)
	}()
	go func() {
		wg.Wait()
		clientSessCh.Close()
		slog.DebugContext(ctx, "Client channel closed.")
	}()

	go s.copyRequests(ctx, gitSession, ghSessCh, clientSessReqs, "client", nil)
	go s.copyRequests(ctx, gitSession, clientSessCh, ghSessReqs, "GitHub", &wg)
	return nil
}

type clientToServer struct {
	gitSession *gitSession

	// TODO avoid this caching
	cachedPayload []byte
}

func (c *clientToServer) Write(p []byte) (int, error) {
	slog.DebugContext(context.Background(), "   client -> GitHub packp", "payload", string(p))
	c.cachedPayload = append(c.cachedPayload, p...)
	return len(p), nil
}

func (c *clientToServer) tryAudit(ctx context.Context, emitter apievents.Emitter) {
	switch c.gitSession.command {
	case gittransport.UploadPackServiceName:
		// TODO Check error.
		emitter.EmitAuditEvent(ctx, &apievents.AppSessionGitFetchRequest{
			Metadata: apievents.Metadata{
				Type:        events.AppSessionGitFetchRequest,
				Code:        events.AppSessionGitFetchRequestCode,
				ClusterName: c.gitSession.identity.RouteToApp.ClusterName,
			},
			UserMetadata: c.gitSession.identity.GetUserMetadata(),
			AppMetadata:  *common.MakeAppMetadata(c.gitSession.app),
			Path:         c.gitSession.path,
		})

	case gittransport.ReceivePackServiceName:
		event := &apievents.AppSessionGitPushRequest{
			Metadata: apievents.Metadata{
				Type:        events.AppSessionGitPushRequest,
				Code:        events.AppSessionGitPushRequestCode,
				ClusterName: c.gitSession.identity.RouteToApp.ClusterName,
			},
			UserMetadata: c.gitSession.identity.GetUserMetadata(),
			AppMetadata:  *common.MakeAppMetadata(c.gitSession.app),
			Path:         c.gitSession.path,
		}

		// More that just 0000
		if len(c.cachedPayload) > 4 {
			request := packp.NewReferenceUpdateRequest()
			err := request.Decode(bytes.NewReader(c.cachedPayload))
			if err == nil {
				for _, command := range request.Commands {
					switch command.Action() {
					case packp.Create:
						slog.DebugContext(ctx, "[===AUDIT===] user created a new reference.", "user", c.gitSession.identity.Username, "reference", command.Name, "to", command.New)
					case packp.Update:
						slog.DebugContext(ctx, "[===AUDIT===] user updated a reference.", "user", c.gitSession.identity.Username, "reference", command.Name, "from", command.Old, "to", command.New)
					case packp.Delete:
						slog.DebugContext(ctx, "[===AUDIT===] user deleted a reference.", "user", c.gitSession.identity.Username, "reference", command.Name, "from", command.Old)
					default:
						slog.DebugContext(ctx, "   client -> GitHub packp.NewReferenceUpdateRequest command", "command", command)
					}

					event.Commands = append(event.Commands, &apievents.AppSessionGitCommand{
						Action:    string(command.Action()),
						Old:       command.Old.String(),
						New:       command.New.String(),
						Reference: string(command.Name),
					})
				}
			} else {
				slog.DebugContext(ctx, "   client -> GitHub packp.NewReferenceUpdateRequest", "request", request, "error", err)
			}
		}
		emitter.EmitAuditEvent(ctx, event)
	}
}

type serverToClient struct {
}

func (s *serverToClient) Write(p []byte) (int, error) {
	slog.DebugContext(context.Background(), "   GitHub -> client packp", "payload", string(p))
	return len(p), nil
}

func parseGitCommand(sshPayload []byte) (string, string, error) {
	sshCommand := struct {
		Command string
	}{}
	if err := ssh.Unmarshal(sshPayload, &sshCommand); err != nil {
		return "", "", trace.Wrap(err)
	}
	gitCommand, path, ok := strings.Cut(sshCommand.Command, " '")
	if !ok {
		return "", "", trace.BadParameter("invalid git command %s", sshCommand.Command)
	}

	if strings.HasSuffix(path, "'") {
		path = strings.TrimSuffix(path, "'")
	} else {
		return "", "", trace.BadParameter("invalid git command %s", sshCommand.Command)
	}

	return gitCommand, path, nil
}

func (s *gitServer) copyRequests(ctx context.Context, session *gitSession, ch ssh.Channel, requests <-chan *ssh.Request, from string, wg *sync.WaitGroup) {
	for req := range requests {
		slog.DebugContext(ctx, "Received request.", "from", from, "type", req.Type, "payload", string(req.Payload))

		if req.Type == "exec" {
			var err error
			session.command, session.path, err = parseGitCommand(req.Payload)
			if err != nil {
				slog.DebugContext(ctx, "Failed to parse exec payload.", "error", err)
				return
			}
			// TODO validate organization from path.
		}

		status, err := ch.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			slog.DebugContext(ctx, "Failed to send request.", "error", err)
			return
		}

		slog.DebugContext(ctx, "    -> Received response.", "status", status, "payload", string(req.Payload))

		err = req.Reply(status, nil)
		if err != nil {
			slog.DebugContext(ctx, "Failed to send reply.", "error", err)
			return
		}

		if req.Type == "exit-status" {
			slog.DebugContext(ctx, "Client disconnected.")
			// TODO Find a better place to close. Closing here sometimes too early before all data are transferred.
			// ch.Close()
			if wg != nil {
				wg.Done()
			}
		}
	}
}

func rejectChannels(ctx context.Context, chans <-chan ssh.NewChannel, from string) {
	for ch := range chans {
		slog.DebugContext(ctx, "Rejecting channel.", "form", from, "type", ch.ChannelType())
		ch.Reject(ssh.UnknownChannelType, "unsupported channel type")
	}
}

func copyGlobalRequests(ctx context.Context, conn ssh.Conn, requests <-chan *ssh.Request, from string) {
	for req := range requests {
		slog.DebugContext(ctx, "Received global request.", "from", from, "type", req.Type, "payload", string(req.Payload))

		status, payload, err := conn.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			slog.DebugContext(ctx, "Failed to send request from client to GitHub.", "error", err)
			return
		}

		slog.DebugContext(ctx, "    -> Received global response.", "status", status, "payload", string(payload))

		err = req.Reply(status, nil)
		if err != nil {
			slog.DebugContext(ctx, "Failed to send reply from GitHub to client.", "error", err)
			return
		}
	}
}
