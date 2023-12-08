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

package benchmark

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

// WebSSHBenchmark is a benchmark suite that connects to the configured
// target hosts via the web api and executes the provided command.
type WebSSHBenchmark struct {
	// Command to execute on the host.
	Command []string
	// Random whether to connect to a random host or not
	Random bool
	// Duration of the test used to determine if renewing web sessions
	// is necessary.
	Duration time.Duration
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (s WebSSHBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (WorkloadFunc, error) {
	clt, sess, err := tc.LoginWeb(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webSess := &webSession{
		webSession: sess,
		clt:        clt,
	}

	// The web session will expire before the duration of the test
	// so launch the renewal loop.
	if !time.Now().Add(s.Duration).Before(webSess.expires()) {
		go webSess.renew(ctx)
	}

	// Add "exit" to ensure that the session terminates after running the command.
	command := strings.Join(append(s.Command, "\r\nexit\r\n"), " ")

	if s.Random {
		if tc.Host != "all" {
			return nil, trace.BadParameter("random ssh bench commands must use the format <user>@all <command>")
		}

		servers, err := getServers(ctx, tc)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return func(ctx context.Context) error {
			return trace.Wrap(s.runCommand(ctx, tc, webSess, chooseRandomHost(servers), command))
		}, nil
	}

	return func(ctx context.Context) error {
		return trace.Wrap(s.runCommand(ctx, tc, webSess, tc.Host, command))
	}, nil
}

type webSession struct {
	mu         sync.Mutex
	webSession types.WebSession
	clt        *client.WebClient
}

func (s *webSession) renew(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(s.expires().Add(-3 * time.Minute))):
			resp, err := s.clt.PostJSON(ctx, s.clt.Endpoint("webapi", "sessions", "renew"), nil)
			if err != nil {
				continue
			}

			session, err := client.GetSessionFromResponse(resp)
			if err != nil {
				continue
			}

			s.mu.Lock()
			s.webSession = session
			s.mu.Unlock()
		}
	}
}

// runCommand starts a non-interactive SSH session and executes the provided
// command before terminating the session.
func (s WebSSHBenchmark) runCommand(ctx context.Context, tc *client.TeleportClient, webSess *webSession, host, command string) error {
	stream, err := connectToHost(ctx, tc, webSess, host)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stream.Close()

	if _, err := io.WriteString(stream, command); err != nil {
		return trace.Wrap(err)
	}

	if _, err := io.Copy(tc.Stdout, stream); err != nil && !errors.Is(err, io.EOF) {
		return trace.Wrap(err)
	}

	return nil
}

// connectToHost opens an SSH session to the target host via the Proxy web api.
func connectToHost(ctx context.Context, tc *client.TeleportClient, webSession *webSession, host string) (io.ReadWriteCloser, error) {
	req := web.TerminalRequest{
		Server: host,
		Login:  tc.HostLogin,
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u := url.URL{
		Host:   tc.WebProxyAddr,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/connect", tc.SiteName),
		RawQuery: url.Values{
			"params":                        []string{string(data)},
			roundtrip.AccessTokenQueryParam: []string{webSession.getToken()},
		}.Encode(),
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: tc.InsecureSkipVerify},
		Jar:             webSession.getCookieJar(),
	}

	ws, resp, err := dialer.DialContext(ctx, u.String(), http.Header{
		"Origin": []string{"http://localhost"},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	ty, _, err := ws.ReadMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ty != websocket.BinaryMessage {
		return nil, trace.BadParameter("unexpected websocket message received %d", ty)
	}

	stream := web.NewTerminalStream(ctx, ws, utils.NewLogger())
	return stream, trace.Wrap(err)
}

// getServers returns all [types.Server] that the authenticated user has
// access to.
func getServers(ctx context.Context, tc *client.TeleportClient) ([]types.Server, error) {
	clt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	resources, err := apiclient.GetAllResources[types.Server](ctx, clt.AuthClient, tc.ResourceFilter(types.KindNode))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(resources) == 0 {
		return nil, trace.BadParameter("no target hosts available")
	}

	return resources, nil
}

func (s *webSession) expires() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.webSession.GetBearerTokenExpiryTime()
}

func (s *webSession) getCookieJar() http.CookieJar {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.clt.HTTPClient().Jar
}

func (s *webSession) getToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.webSession.GetBearerToken()
}
