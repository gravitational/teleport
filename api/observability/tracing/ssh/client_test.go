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

	"github.com/gravitational/trace"
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
			errChan := make(chan error, 5)

			srv := newServer(t, tt.expectedCapability, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
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
							errChan <- trace.Wrap(err, "rejecting channel")
							return
						}
					}
				}
			})

			if tt.srvVersion != "" {
				srv.config.ServerVersion = tt.srvVersion
			}

			go srv.Run(errChan)

			conn, chans, reqs := srv.GetClient(t)
			client := NewClient(conn, chans, reqs)

			require.Equal(t, tt.expectedCapability, client.capability)

			select {
			case err := <-errChan:
				require.NoError(t, err)
			default:
			}
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
	errChan := make(chan error, 5)

	expected := map[string]string{"a": "1", "b": "2", "c": "3"}

	// used to collect individual envs requests
	envReqC := make(chan envReqParams, 3)

	srv := newServer(t, tracingSupported, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
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
						errChan <- trace.Wrap(err, "failed to accept session channel")
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
									errChan <- err
									return
								}
							}
						}
					}()
				default:
					if err := ch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unexpected channel %s", ch.ChannelType())); err != nil {
						errChan <- err
						return
					}
				}
			}
		}
	})

	go srv.Run(errChan)

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

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}
}
