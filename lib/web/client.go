// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/session"
)

type ProxyClient struct {
	clt     *client.WebClient
	jar     http.CookieJar
	url     url.URL
	session types.WebSession
}

func NewProxyClient(u url.URL, session types.WebSession, cookies []*http.Cookie) (*ProxyClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	jar.SetCookies(&u, cookies)

	clt, err := client.NewWebClient(u.String(), roundtrip.BearerAuth(session.GetBearerToken()), roundtrip.CookieJar(jar))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ProxyClient{
		clt:     clt,
		jar:     jar,
		session: session,
		url:     u,
	}, nil
}

func (p *ProxyClient) connectToHost(ctx context.Context, sess session.Session, tc *client.TeleportClient) (*websocket.Conn, error) {
	type siteSession struct {
		Session session.Session `json:"session"`
	}

	resp, err := p.clt.PostJSON(ctx, p.clt.Endpoint("webapi", "sites", tc.SiteName, "sessions"), siteSession{Session: sess})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var created siteSession
	if err := json.Unmarshal(resp.Bytes(), &created); err != nil {
		return nil, trace.Wrap(err)
	}

	wsu := &url.URL{
		Host:   p.url.Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/connect", tc.SiteName),
	}
	data, err := json.Marshal(TerminalRequest{
		Server: sess.ServerID,
		Login:  sess.Login,
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
		SessionID:          created.Session.ID,
		InteractiveCommand: make([]string, 1),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	q := wsu.Query()
	q.Set("params", string(data))
	q.Set(roundtrip.AccessTokenQueryParam, p.session.GetBearerToken())
	wsu.RawQuery = q.Encode()

	dialer := websocket.Dialer{
		Jar: p.jar,
	}

	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	ws, wsresp, err := dialer.DialContext(dialCtx, wsu.String(), http.Header{"Origin": []string{"http://localhost"}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer wsresp.Body.Close()

	return ws, nil
}

func (p *ProxyClient) SSH(ctx context.Context, tc *client.TeleportClient, command []string, interactive bool) error {
	// detect the common error when users use host:port address format
	_, port, err := net.SplitHostPort(tc.Host)
	// client has used host:port notation
	if err == nil {
		return trace.BadParameter(
			"please use ssh subcommand with '--port=%v' flag instead of semicolon",
			port)
	}

	sess := session.Session{
		Login:          tc.HostLogin,
		ServerID:       net.JoinHostPort(tc.Host, strconv.Itoa(tc.HostPort)),
		TerminalParams: session.TerminalParams{W: 100, H: 100},
	}

	conn, err := p.connectToHost(ctx, sess, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyClient, err := tc.ConnectToProxy(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	authClt, err := proxyClient.ConnectToRootCluster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	wsess, err := authClt.GetWebSession(ctx, types.GetWebSessionRequest{
		User:      p.session.GetUser(),
		SessionID: p.session.GetName(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	sessCtx, err := NewSessionContext(wsess, authClt)
	if err != nil {
		return trace.Wrap(err)
	}

	th, err := NewTerminal(
		ctx,
		TerminalRequest{
			Server: tc.HostLogin,
			Login:  tc.Username,
			Term: session.TerminalParams{
				W: 100,
				H: 100,
			},
			SessionID:          session.NewID(),
			Namespace:          apidefaults.Namespace,
			ProxyHostPort:      tc.WebProxyAddr,
			Cluster:            tc.SiteName,
			InteractiveCommand: make([]string, 1),
		},
		authClt,
		sessCtx,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer th.Close()

	stream := th.asTerminalStream(conn)
	defer stream.Close()

	cmd := strings.Join(command, " ") + "\r\n"
	if !interactive {
		cmd += "exit\r\n"
	}

	if _, err := stream.Write([]byte(cmd)); err != nil {
		log.WithError(err).Warn("--------------- failed to execute command")
	}

	if _, err := io.Copy(io.Discard, stream); err != nil && !errors.Is(err, net.ErrClosed) {
		log.WithError(err).Warn("--------- failed to copy output to stream")
	}

	return nil

}
