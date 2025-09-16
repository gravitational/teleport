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
	"os"
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
	"github.com/gravitational/teleport/lib/web/terminal"
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
	// The benchmark runner may override stderr to be [io.Discard] which
	// results in the login prompt being sent into the void and the user
	// staring at a blank terminal. Temporarily override stderr to allow
	// the prompt to be written to the terminal.
	stderr := tc.Stderr
	tc.Stderr = os.Stderr

	clt, sess, err := tc.LoginWeb(ctx)
	if err != nil {
		tc.Stderr = stderr
		return nil, trace.Wrap(err)
	}

	tc.Stderr = stderr
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

// TerminalRequest describes a request to create a web-based terminal
// to a remote SSH server.
type TerminalRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port]).
	Server string `json:"server_id"`

	// Login is Linux username to connect as.
	Login string `json:"login"`

	// Term is the initial PTY size.
	Term session.TerminalParams `json:"term"`
}

// connectToHost opens an SSH session to the target host via the Proxy web api.
func connectToHost(ctx context.Context, tc *client.TeleportClient, webSession *webSession, host string) (io.ReadWriteCloser, error) {
	req := TerminalRequest{
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
		Path:   fmt.Sprintf("/v1/webapi/sites/%v/connect/ws", tc.SiteName),
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

	stream := terminal.NewStream(ctx, terminal.StreamConfig{WS: ws})
	return stream, trace.Wrap(err)
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

// WebSessionBenchmark is a benchmark suite that connects to the configured
// target hosts via the web api and executes the provided command.
type WebSessionBenchmark struct {
	// Command to execute on the host.
	Command []string
	// Max number of sessions to have open at once.
	Max int
	// Duration of the test used to determine if renewing web sessions
	// is necessary.
	Duration time.Duration

	servers []types.Server
}

func (s *WebSessionBenchmark) ConfigOverride(ctx context.Context, tc *client.TeleportClient, cfg *Config) error {
	servers, err := getServers(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	s.servers = servers

	if s.Max == 0 {
		s.Max = len(servers)
	}

	// alter the minimum window such that the test will run
	// for as long as is required to spawn all the sessions plus
	// an additional minute.
	interval := time.Duration(1 / float64(cfg.Rate) * float64(time.Second))
	window := interval*time.Duration(s.Max) + (time.Minute)
	if cfg.MinimumWindow < window {
		cfg.MinimumWindow = window
	}

	return nil
}

// BenchBuilder returns a WorkloadFunc for the given benchmark suite.
func (s *WebSessionBenchmark) BenchBuilder(ctx context.Context, tc *client.TeleportClient) (WorkloadFunc, error) {
	// The benchmark runner may override stderr to be [io.Discard] which
	// results in the login prompt being sent into the void and the user
	// staring at a blank terminal. Temporarily override stderr to allow
	// the prompt to be written to the terminal.
	stderr := tc.Stderr
	tc.Stderr = os.Stderr

	clt, sess, err := tc.LoginWeb(ctx)
	if err != nil {
		tc.Stderr = stderr
		return nil, trace.Wrap(err)
	}

	tc.Stderr = stderr

	webSess := &webSession{
		webSession: sess,
		clt:        clt,
	}

	// The web session will expire before the duration of the test
	// so launch the renewal loop.
	if !time.Now().Add(s.Duration).Before(webSess.expires()) {
		go webSess.renew(ctx)
	}

	var (
		mu     sync.Mutex
		active int
		next   int
	)

	// Open a ssh session to the next host if the maximum
	// number of connections has not already been reached.
	return func(ctx context.Context) error {
		mu.Lock()
		if active >= s.Max {
			mu.Unlock()
			return nil
		}
		active++

		current := next
		next = (next + 1) % len(s.servers)
		mu.Unlock()

		defer func() {
			mu.Lock()
			active--
			mu.Unlock()
		}()

		stream, err := connectToHost(ctx, tc, webSess, s.servers[current].GetName()+":0")
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(utils.ProxyConn(ctx,
			&streamCloser{
				ReadWriteCloser: stream,
			},
			rwc{
				r: repeatingReader{
					ctx:      ctx,
					s:        strings.Join(append(s.Command, "\r\n"), " "),
					interval: time.Second,
				},
				w: tc.Stdout,
				c: io.NopCloser(stream),
			}))
	}, nil
}

// streamCloser allows the client to end the
// session by sending "exit" to the server. This
// allows all sessions initiated by the benchmark to
// disappear immediately instead of having them linger
// until the session tracker is expired.
type streamCloser struct {
	io.ReadWriteCloser
	once sync.Once
}

func (s *streamCloser) Close() error {
	var err error
	s.once.Do(func() {
		_, exitErr := s.ReadWriteCloser.Write([]byte("\r\nexit\r\n"))
		err = trace.NewAggregate(exitErr, s.ReadWriteCloser.Close())
	})

	return trace.Wrap(err)
}

type rwc struct {
	r io.Reader
	w io.Writer
	c io.Closer
}

func (r rwc) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r rwc) Write(p []byte) (int, error) {
	return r.w.Write(p)
}

func (r rwc) Close() error {
	return r.c.Close()
}

// repeatingReader is an [io.Reader] that periodically
// returns the same value until the context is
// terminated.
type repeatingReader struct {
	ctx      context.Context
	s        string
	interval time.Duration
}

func (r repeatingReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	select {
	case <-r.ctx.Done():
		return 0, io.EOF
	case <-time.After(r.interval):
	}

	end := len(r.s)
	if end > len(p) {
		end = len(p)
	}

	n := copy(p, r.s[:end])
	return n, nil
}
