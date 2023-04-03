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
	rejected := &ssh.OpenChannelError{
		Reason:  ssh.Prohibited,
		Message: "rejected!",
	}

	unknown := &ssh.OpenChannelError{
		Reason:  ssh.UnknownChannelType,
		Message: "unknown!",
	}

	cases := []struct {
		name               string
		channelErr         *ssh.OpenChannelError
		srvVersion         string
		expectedCapability tracingCapability
		errAssertion       require.ErrorAssertionFunc
	}{
		{
			name:               "rejected",
			channelErr:         rejected,
			expectedCapability: tracingUnknown,
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.Equal(t, rejected.Error(), err.Error())
			},
		},
		{
			name:               "unknown",
			channelErr:         unknown,
			expectedCapability: tracingUnsupported,
			errAssertion:       require.NoError,
		},
		{
			name:               "supported",
			channelErr:         nil,
			expectedCapability: tracingSupported,
			errAssertion:       require.NoError,
		},
		{
			name:               "no backend support",
			channelErr:         nil,
			expectedCapability: tracingUnsupported,
			srvVersion:         "SSH-2.0-OpenSSH_7.4", // Only Teleport supports tracing
			errAssertion:       require.NoError,
		},
		{
			name: "other error",
			channelErr: &ssh.OpenChannelError{
				Reason:  ssh.ConnectionFailed,
				Message: "",
			},
			expectedCapability: tracingUnknown,
			errAssertion:       require.NoError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			errChan := make(chan error, 5)

			srv := newServer(t, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
				for {
					select {
					case <-ctx.Done():
						return

					case ch := <-channels:
						if ch == nil {
							return
						}

						if tt.channelErr != nil {
							if err := ch.Reject(tt.channelErr.Reason, tt.channelErr.Message); err != nil {
								errChan <- trace.Wrap(err, "failed to reject channel")
							}
							return
						}

						_, _, err := ch.Accept()
						if err != nil {
							errChan <- trace.Wrap(err, "failed to accept channel")
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
			client := ssh.NewClient(conn, chans, reqs)

			capabaility, err := isTracingSupported(client)
			require.Equal(t, tt.expectedCapability, capabaility)
			tt.errAssertion(t, err)

			select {
			case err := <-errChan:
				require.NoError(t, err)
			default:
			}
		})
	}
}

func TestNewSession(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	errChan := make(chan error, 5)

	first := ssh.OpenChannelError{
		Reason:  ssh.Prohibited,
		Message: "first attempt",
	}

	second := ssh.OpenChannelError{
		Reason:  ssh.ConnectionFailed,
		Message: "second attempt",
	}

	srv := newServer(t, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				return

			case ch := <-channels:
				switch {
				case ch == nil:
					return
				case ch.ChannelType() == "session":
					_, _, err := ch.Accept()
					if err != nil {
						errChan <- trace.Wrap(err, "failed to accept session channel")
						return
					}
				case i == 0:
					if err := ch.Reject(first.Reason, first.Message); err != nil {
						errChan <- err
						return
					}
				case i == 1:
					if err := ch.Reject(second.Reason, second.Message); err != nil {
						errChan <- err
						return
					}
				case i > 2:
					if _, _, err := ch.Accept(); err != nil {
						errChan <- err
						return
					}
				default:
					if err := ch.Reject(ssh.ConnectionFailed, fmt.Sprintf("unexpected channel %d", i)); err != nil {
						errChan <- err
						return
					}
				}
			}
		}
	})

	go srv.Run(errChan)

	cases := []struct {
		name          string
		assertionFunc func(t *testing.T, clt *Client, session *Session, err error)
	}{
		{
			name: "session prohibited",
			assertionFunc: func(t *testing.T, clt *Client, sess *Session, err error) {
				// creating a new session should return any errors captured when creating the client
				// and not actually probe the server
				require.Error(t, err)
				require.Equal(t, trace.Unwrap(err).Error(), first.Error())
				require.Nil(t, sess)
				require.Nil(t, clt.rejectedError)
				require.Equal(t, clt.capability, tracingUnknown)
			},
		},
		{
			name: "other failure to open tracing channel",
			assertionFunc: func(t *testing.T, clt *Client, sess *Session, err error) {
				// this time through we should probe the server without getting a prohibited error,
				// but things still failed, so we shouldn't know the capability
				require.NoError(t, err)
				require.NotNil(t, sess)
				require.NoError(t, clt.rejectedError)
				require.Equal(t, clt.capability, tracingUnknown)
				require.NoError(t, sess.Close())
			},
		},
		{
			name: "active session",
			assertionFunc: func(t *testing.T, clt *Client, sess *Session, err error) {
				// all is good now, we should have an active session
				require.NoError(t, err)
				require.NotNil(t, sess)
				require.NoError(t, clt.rejectedError)
				require.Equal(t, clt.capability, tracingSupported)
				require.NoError(t, sess.Close())
			},
		},
	}

	// check tracing status after first capability probe from creating the client
	conn, chans, reqs := srv.GetClient(t)
	client := NewClient(conn, chans, reqs)
	require.Error(t, client.rejectedError)
	require.Equal(t, client.rejectedError.Error(), first.Error())
	require.Equal(t, client.capability, tracingUnknown)

	select {
	case err := <-errChan:
		require.NoError(t, err)
	default:
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			sess, err := client.NewSession(ctx)
			tt.assertionFunc(t, client, sess, err)

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

	srv := newServer(t, func(conn *ssh.ServerConn, channels <-chan ssh.NewChannel, requests <-chan *ssh.Request) {
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
