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

package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestIsTracingSupported(t *testing.T) {
	cases := []struct {
		name               string
		srvVersion         string
		expectedCapability tracingCapability
	}{
		{
			name:               "supported",
			expectedCapability: tracingSupported,
			srvVersion:         "SSH-2.0-Teleport",
		},
		{
			name:               "unsupported",
			expectedCapability: tracingUnsupported,
			srvVersion:         "SSH-2.0-OpenSSH_7.4", // Only Teleport supports tracing
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			srv := newServer(t, tt.srvVersion, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
				go ssh.DiscardRequests(requests)

				for {
					select {
					case <-ctx.Done():
						return

					case ch := <-channels:
						if ch == nil {
							return
						}

						if err := ch.Reject(ssh.Prohibited, "no channels allowed"); err != nil {
							assert.NoError(t, err, "rejecting channel")
							return
						}
					}
				}
			})

			conn, chans, reqs := srv.GetClient(t)
			client := NewClient(conn, chans, reqs)

			require.Equal(t, tt.expectedCapability, client.capability)
		})
	}
}

// envReqParams are parameters for env request
type envReqParams struct {
	Name  string
	Value string
}

// TestSetEnvs verifies that client uses EnvsRequest to
// send multiple envs and falls back to sending individual "env"
// requests if the server does not support EnvsRequests.
func TestSetEnvs(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	expected := map[string]string{"a": "1", "b": "2", "c": "3"}

	// used to collect individual envs requests
	envReqC := make(chan envReqParams, 3)

	srv := newServer(t, tracingSupportedVersion, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
		for {
			select {
			case <-ctx.Done():
				return
			case ch := <-channels:
				switch {
				case ch == nil:
					return
				case ch.ChannelType() == "session":
					ch, reqs, err := ch.Accept()
					if err != nil {
						assert.NoError(t, err, "failed to accept session channel")
						return
					}

					go func() {
						defer ch.Close()
						for i := 0; ; i++ {
							select {
							case <-ctx.Done():
								return
							case req := <-reqs:
								if req == nil {
									return
								}

								switch {
								case i == 0 && req.Type == EnvsRequest: // accept 1st EnvsRequest
									var envReq EnvsReq
									if err := ssh.Unmarshal(req.Payload, &envReq); err != nil {
										_ = req.Reply(false, []byte(err.Error()))
										return
									}

									var envs map[string]string
									if err := json.Unmarshal(envReq.EnvsJSON, &envs); err != nil {
										_ = req.Reply(false, []byte(err.Error()))
										return
									}

									for k, v := range expected {
										actual, ok := envs[k]
										if !ok {
											_ = req.Reply(false, []byte(fmt.Sprintf("expected env %s not present", k)))
											return
										}

										if actual != v {
											_ = req.Reply(false, []byte(fmt.Sprintf("expected value %s for env %s, got %s", v, k, actual)))
											return
										}
									}

									_ = req.Reply(true, nil)
								case i == 1 && req.Type == EnvsRequest: // reject additional EnvsRequest so we test fallbacks
									_ = req.Reply(false, nil)
								case i >= 2 && i <= len(expected)+2 && req.Type == "env": // accept individual "env" fallbacks.
									var e envReqParams
									if err := ssh.Unmarshal(req.Payload, &e); err != nil {
										_ = req.Reply(false, []byte(err.Error()))
										return
									}
									envReqC <- e
									_ = req.Reply(true, nil)
								default: // out of order or unexpected message
									_ = req.Reply(false, []byte(fmt.Sprintf("unexpected ssh request %s on iteration %d", req.Type, i)))
									assert.NoError(t, err)
									return
								}
							}
						}
					}()
				default:
					if err := ch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unexpected channel %s", ch.ChannelType())); err != nil {
						assert.NoError(t, err)
						return
					}
				}
			}
		}
	})

	// create a client and open a session
	conn, chans, reqs := srv.GetClient(t)
	client := NewClient(conn, chans, reqs)
	session, err := client.NewSession(ctx)
	require.NoError(t, err)

	// the first request shouldn't fall back
	t.Run("envs set via envs@goteleport.com", func(t *testing.T) {
		require.NoError(t, session.SetEnvs(ctx, expected))

		select {
		case <-envReqC:
			t.Fatal("env request received instead of an envs@goteleport.com request")
		default:
		}
	})

	// subsequent requests should fall back to standard "env" requests
	t.Run("envs set individually", func(t *testing.T) {
		require.NoError(t, session.SetEnvs(ctx, expected))

		envs := map[string]string{}
		envsTimeout := time.NewTimer(3 * time.Second)
		defer envsTimeout.Stop()
		for i := 0; i < len(expected); i++ {
			select {
			case env := <-envReqC:
				envs[env.Name] = env.Value
			case <-envsTimeout.C:
				t.Fatalf("Time out waiting for env request %d to be processed", i)
			}
		}

		for k, v := range expected {
			actual, ok := envs[k]
			require.True(t, ok, "expected env %s to be set", k)
			require.Equal(t, v, actual, "expected value %s for env %s, got %s", v, k, actual)
		}
	})
}

type mockSSHChannel struct {
	ssh.Channel
}

func TestWrappedSSHConn(t *testing.T) {
	sshCh := new(mockSSHChannel)
	reqs := make(<-chan *ssh.Request)

	// ensure that OpenChannel returns the same SSH channel and requests
	// chan that wrappedSSHConn was given
	wrappedConn := &wrappedSSHConn{
		ch:   sshCh,
		reqs: reqs,
	}
	retCh, retReqs, err := wrappedConn.OpenChannel("", nil)
	require.NoError(t, err)
	require.Equal(t, sshCh, retCh)
	require.Equal(t, reqs, retReqs)

	// ensure the wrapped SSH conn will panic if OpenChannel is called
	// twice
	require.Panics(t, func() {
		wrappedConn.OpenChannel("", nil)
	})
}

// TestGlobalAndSessionRequests tests that the tracing client correctly handles global and session requests.
func TestGlobalAndSessionRequests(t *testing.T) {
	ctx := t.Context()

	// pingRequest is an example request type. Whether sent by the server or client in
	// a global or session context, the receiver should give an ok as the reply.
	pingRequest := "ping@goteleport.com"

	clientGlobalReply := make(chan bool, 1)
	clientSessionReply := make(chan bool, 1)

	srv := newServer(t, tracingSupportedVersion, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
		// Send a ping request when the client connection is established.
		ok, _, err := conn.SendRequest(pingRequest, true, nil)
		assert.NoError(t, err, "server failed to send global ping request")
		clientGlobalReply <- ok

		for {
			select {
			case <-ctx.Done():
				return
			case req := <-requests:
				switch req.Type {
				case pingRequest:
					err := req.Reply(true, nil)
					assert.NoError(t, err, "server failed to reply to global ping request")
				default:
					err := req.Reply(false, nil)
					assert.NoError(t, err, "server failed to reply to global %q request", req.Type)
				}
			case ch := <-channels:
				switch {
				case ch == nil:
					return
				case ch.ChannelType() == "session":
					ch, reqs, err := ch.Accept()
					if err != nil {
						assert.NoError(t, err, "failed to accept session channel")
						return
					}

					go func() {
						defer ch.Close()
						for {
							select {
							case <-ctx.Done():
								return
							case req := <-reqs:
								switch req.Type {
								case pingRequest:
									err := req.Reply(true, nil)
									assert.NoError(t, err, "server failed to reply to session ping request")
								}
								continue
							}
						}
					}()

					// Send a ping request when the session is established.
					ok, err := ch.SendRequest(pingRequest, true, nil)
					assert.NoError(t, err, "server failed to send ping request")
					clientSessionReply <- ok
				default:
					err := ch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unexpected channel %s", ch.ChannelType()))
					assert.NoError(t, err)
				}
			}
		}
	})

	conn, chans, reqs := srv.GetClient(t)
	client := NewClient(conn, chans, reqs)

	// The client should reply false to any global request from the server, as we
	// don't currently support a mechanism for the client to register global handlers.
	select {
	case reply := <-clientGlobalReply:
		require.False(t, reply, "Expected the client to reply false to global ping request")
	case <-time.After(10 * time.Second):
		t.Fatalf("Failed to receive client global reply to ping request")
	}

	// The server should reply true to a global ping request.
	ok, _, err := client.SendRequest(ctx, pingRequest, true, nil)
	require.True(t, ok, "Expected the server to reply true to global ping request")
	require.NoError(t, err)

	// If the client isn't setup to handle session requests, it should reply false to them.
	// The client should reply true to a session ping request.
	_, err = client.NewSession(ctx)
	require.NoError(t, err)

	select {
	case reply := <-clientSessionReply:
		require.False(t, reply, "Expected the client to reply false to session ping request")
	case <-time.After(10 * time.Second):
		t.Fatalf("Failed to receive client session reply to ping request")
	}

	// The client should reply true to a session ping request.
	err = client.HandleSessionRequest(ctx, pingRequest, func(ctx context.Context, req *ssh.Request) {
		err := req.Reply(true, nil)
		assert.NoError(t, err)
	})
	require.NoError(t, err)
	_, err = client.NewSession(ctx)
	require.NoError(t, err)

	select {
	case reply := <-clientSessionReply:
		require.True(t, reply, "Expected the client to reply true to session ping request")
	case <-time.After(10 * time.Second):
		t.Fatalf("Failed to receive client session reply to ping request")
	}

	// New Sessions do not reuse previously registered handlers.
	session, err := client.NewSession(ctx)
	require.NoError(t, err)

	select {
	case reply := <-clientSessionReply:
		require.False(t, reply, "Expected the client to reply false to session ping request")
	case <-time.After(10 * time.Second):
		t.Fatalf("Failed to receive client session reply to ping request")
	}

	// The server should reply true to a session ping request.
	ok, err = session.SendRequest(ctx, pingRequest, true, nil)
	require.NoError(t, err)
	require.True(t, ok, "Expected the server to reply true to session ping request")
}
