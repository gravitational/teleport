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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/session"
)

type ShellCreatedCallback func(terminal io.ReadWriteCloser) error

type ProxyClient struct {
	clt *client.WebClient
	jar http.CookieJar
	url url.URL

	lock    sync.Mutex
	session types.WebSession
	expiry  time.Time
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

	pc := &ProxyClient{
		clt:     clt,
		jar:     jar,
		session: session,
		url:     u,
		expiry:  session.GetBearerTokenExpiryTime().Add(-1 * time.Minute),
	}

	//go pc.renewSession(context.Background())

	return pc, nil
}

func (p *ProxyClient) renewSession(ctx context.Context) {
	for {
		if p.expiry.Before(time.Now()) {
			<-time.After(p.expiry.Sub(time.Now()))
		}

		logrus.Info("---------- attempting to renew session")

		resp, err := p.clt.PostJSON(ctx, p.clt.Endpoint("webapi", "sessions", "renew"), struct{}{})
		if err != nil {
			logrus.WithError(err).Warn("failed to renew web session")
			continue
		}

		var sess CreateSessionResponse
		err = json.Unmarshal(resp.Bytes(), &sess)
		if err != nil {
			logrus.WithError(err).Warn("failed to renew web session")
			continue
		}

		// SessionCookie stores information about active user and session
		type SessionCookie struct {
			User string `json:"user"`
			SID  string `json:"sid"`
		}

		cookies := resp.Cookies()
		if len(cookies) != 1 {
			logrus.WithError(err).Warn("cookie is missing")
			continue
		}

		var cookie SessionCookie
		if err := json.NewDecoder(hex.NewDecoder(strings.NewReader(cookies[0].Value))).Decode(&cookie); err != nil {
			logrus.WithError(err).Warn("failed to decode cookie")
			continue
		}

		expiry := time.Now().Add(time.Duration(sess.TokenExpiresIn) * time.Second)
		websess, err := types.NewWebSession(cookie.SID, types.KindWebSession, types.WebSessionSpecV2{
			User:               cookie.User,
			Pub:                p.session.GetPub(),
			BearerToken:        sess.Token,
			BearerTokenExpires: expiry,
			Expires:            expiry,
			LoginTime:          time.Now(),
			IdleTimeout:        types.Duration(time.Duration(sess.SessionInactiveTimeoutMS) * time.Millisecond),
		})

		jar, err := cookiejar.New(nil)
		if err != nil {
			logrus.WithError(err).Warn("failed to create cookie jar")
			continue
		}

		jar.SetCookies(&p.url, cookies)

		clt, err := client.NewWebClient(p.url.String(), roundtrip.BearerAuth(websess.GetBearerToken()), roundtrip.CookieJar(jar))
		if err != nil {
			logrus.WithError(err).Warn("failed to create web client")
			continue
		}

		p.lock.Lock()
		p.clt = clt
		p.jar = jar
		p.session = websess
		p.expiry = websess.GetBearerTokenExpiryTime().Add(-5 * time.Minute)
		p.lock.Unlock()
	}
}

func (p *ProxyClient) connectToHost(ctx context.Context, sess session.Session, tc *client.TeleportClient) (*websocket.Conn, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

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

	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ws, wsresp, err := dialer.DialContext(dialCtx, wsu.String(), http.Header{"Origin": []string{"http://localhost"}})
	if err != nil {
		return nil, trace.Wrap(err, "dialing websocket %s timed out", wsu.String())
	}
	defer wsresp.Body.Close()

	return ws, nil
}

func (p *ProxyClient) CurrentWebSession(ctx context.Context, clt auth.ClientI) (types.WebSession, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	wsess, err := clt.GetWebSession(ctx, types.GetWebSessionRequest{
		User:      p.session.GetUser(),
		SessionID: p.session.GetName(),
	})
	return wsess, trace.Wrap(err)
}

func (p *ProxyClient) SSH(ctx context.Context, tc *client.TeleportClient, node string, command string, authClt auth.ClientI, wsess types.WebSession, keepAliveInterval time.Duration, cb ShellCreatedCallback) error {
	sess := session.Session{
		Login:          tc.Host,
		ServerID:       node,
		TerminalParams: session.TerminalParams{W: 100, H: 100},
	}

	sessCtx, err := NewSessionContext(wsess, authClt)
	if err != nil {
		return trace.Wrap(err)
	}

	cmd := command + "\r\n"
	id := session.NewID()
	th, err := NewTerminal(
		ctx,
		TerminalRequest{
			Server: node,
			Login:  tc.Username,
			Term: session.TerminalParams{
				W: 100,
				H: 100,
			},
			SessionID:          id,
			Namespace:          apidefaults.Namespace,
			ProxyHostPort:      tc.WebProxyAddr,
			Cluster:            tc.SiteName,
			InteractiveCommand: []string{cmd},
			KeepAliveInterval:  keepAliveInterval,
		},
		authClt,
		sessCtx,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer th.Close()

	th.terminalContext, th.terminalCancel = context.WithCancel(ctx)
	defer th.terminalCancel()

	conn, err := p.connectToHost(ctx, sess, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	stream := th.asTerminalStream(conn)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		th.startPingLoop(conn)
		return stream.Close()
	})

	if cb != nil {
		if err := cb(stream); err != nil {
			return trace.Wrap(err)
		}
	}

	g.Go(func() error {
		if _, err := stream.Write([]byte(cmd)); err != nil {
			logrus.WithError(err).Warn("--------------- failed to execute command")
		}

		if _, err := io.Copy(io.Discard, stream); err != nil && !errors.Is(err, net.ErrClosed) {
			logrus.WithError(err).Warn("--------- failed to copy output to stream")
		}

		return nil
	})

	return g.Wait()
}
