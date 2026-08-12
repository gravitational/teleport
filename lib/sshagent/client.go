// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package sshagent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

// Client extends the [agent.ExtendedAgent] interface with an [io.Closer].
type Client interface {
	agent.ExtendedAgent
	io.Closer
}

// ClientGetter is a function used to get a new agent client.
type ClientGetter = func() (Client, error)

type client struct {
	agent.ExtendedAgent
	conn io.Closer
}

// NewClient creates a new SSH Agent client with an open connection using
// the provided connection function. The resulting connection can be any
// [io.ReadWriteCloser], such as a [net.Conn] or [ssh.Channel].
func NewClient(connect func() (io.ReadWriteCloser, error)) (Client, error) {
	conn, err := connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &client{
		ExtendedAgent: agent.NewClient(conn),
		conn:          conn,
	}, nil
}

// NewSystemAgentClient creates a new SSH Agent client with an open connection
// to the system agent, advertised by SSH_AUTH_SOCK or other system parameters.
func NewSystemAgentClient() (Client, error) {
	return NewClient(DialSystemAgent)
}

// NewStaticClient creates a new SSH Agent client for the given static agent.
func NewStaticClient(agentClient agent.ExtendedAgent) Client {
	return &client{
		ExtendedAgent: agentClient,
	}
}

// NewStaticClientGetter returns a [ClientGetter] for a static agent client.
func NewStaticClientGetter(agentClient agent.ExtendedAgent) ClientGetter {
	return func() (Client, error) {
		return &client{
			ExtendedAgent: agentClient,
		}, nil
	}
}

// Close the agent client and prevent further requests.
func (c *client) Close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	return trace.Wrap(err)
}

// NewSingleRequestClient returns a client that opens a new connection for each request
// and closes it once the request completes.
//
// The returned agent is safe for concurrent use as long as getClient is.
func NewSingleRequestClient(getClient ClientGetter) agent.ExtendedAgent {
	return singleRequestClient{getClient: getClient}
}

type singleRequestClient struct {
	getClient ClientGetter
}

// withSingleRequestClient opens a new agent connection, runs fn against it,
// and closes the connection.
func withSingleRequestClient[T any](getClient ClientGetter, fn func(Client) (T, error)) (T, error) {
	agentClient, err := getClient()
	if err != nil {
		var zero T
		return zero, trace.Wrap(err)
	}
	defer agentClient.Close()

	out, err := fn(agentClient)
	return out, trace.Wrap(err)
}

// doWithSingleRequestClient is [withSingleRequestClient] for requests with no return value.
func doWithSingleRequestClient(getClient ClientGetter, fn func(Client) error) error {
	_, err := withSingleRequestClient(getClient, func(agentClient Client) (struct{}, error) {
		return struct{}{}, fn(agentClient)
	})
	return trace.Wrap(err)
}

func (s singleRequestClient) List() ([]*agent.Key, error) {
	return withSingleRequestClient(s.getClient, func(agentClient Client) ([]*agent.Key, error) {
		return agentClient.List()
	})
}

func (s singleRequestClient) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return withSingleRequestClient(s.getClient, func(agentClient Client) (*ssh.Signature, error) {
		return agentClient.Sign(key, data)
	})
}

func (s singleRequestClient) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return withSingleRequestClient(s.getClient, func(agentClient Client) (*ssh.Signature, error) {
		return agentClient.SignWithFlags(key, data, flags)
	})
}

func (s singleRequestClient) Add(key agent.AddedKey) error {
	return doWithSingleRequestClient(s.getClient, func(agentClient Client) error {
		return agentClient.Add(key)
	})
}

func (s singleRequestClient) Remove(key ssh.PublicKey) error {
	return doWithSingleRequestClient(s.getClient, func(agentClient Client) error {
		return agentClient.Remove(key)
	})
}

func (s singleRequestClient) RemoveAll() error {
	return doWithSingleRequestClient(s.getClient, func(agentClient Client) error {
		return agentClient.RemoveAll()
	})
}

func (s singleRequestClient) Lock(passphrase []byte) error {
	return doWithSingleRequestClient(s.getClient, func(agentClient Client) error {
		return agentClient.Lock(passphrase)
	})
}

func (s singleRequestClient) Unlock(passphrase []byte) error {
	return doWithSingleRequestClient(s.getClient, func(agentClient Client) error {
		return agentClient.Unlock(passphrase)
	})
}

func (s singleRequestClient) Extension(extensionType string, contents []byte) ([]byte, error) {
	return withSingleRequestClient(s.getClient, func(agentClient Client) ([]byte, error) {
		return agentClient.Extension(extensionType, contents)
	})
}

// Signers returns signers for all the keys currently known to the agent.
// The signers do not hold a connection open, they connect to the agent on demand
// for each signature.
func (s singleRequestClient) Signers() ([]ssh.Signer, error) {
	keys, err := s.List()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signers := make([]ssh.Signer, 0, len(keys))
	for _, key := range keys {
		signers = append(signers, singleRequestSigner{getClient: s.getClient, pub: key})
	}
	return signers, nil
}

// singleRequestSigner is a signer for a key held by an agent that
// opens a new connection for each signature request.
type singleRequestSigner struct {
	getClient ClientGetter
	pub       ssh.PublicKey
}

var _ ssh.AlgorithmSigner = singleRequestSigner{}

func (s singleRequestSigner) PublicKey() ssh.PublicKey {
	return s.pub
}

func (s singleRequestSigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	// Note: the agent has its own entropy source, so the rand argument is ignored.
	return withSingleRequestClient(s.getClient, func(agentClient Client) (*ssh.Signature, error) {
		return agentClient.Sign(s.pub, data)
	})
}

func (s singleRequestSigner) SignWithAlgorithm(rand io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	return withSingleRequestClient(s.getClient, func(agentClient Client) (*ssh.Signature, error) {
		signer, err := agentSigner(agentClient, s.pub)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return signer.SignWithAlgorithm(rand, data, algorithm)
	})
}

// agentSigner returns the agent's own signer for the given public key.
func agentSigner(agentClient Client, pub ssh.PublicKey) (ssh.AlgorithmSigner, error) {
	signers, err := agentClient.Signers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubBytes := pub.Marshal()
	for _, signer := range signers {
		if !bytes.Equal(signer.PublicKey().Marshal(), pubBytes) {
			continue
		}
		algorithmSigner, ok := signer.(ssh.AlgorithmSigner)
		if !ok {
			return nil, trace.NotImplemented("agent signer of type %T does not support signing with a specific algorithm", signer)
		}
		return algorithmSigner, nil
	}

	return nil, trace.NotFound("agent no longer holds the requested %v key", pub.Type())
}

const channelType = "auth-agent@openssh.com"

// ServeChannelRequests routes agent channel requests to a new agent
// connection retrieved from the provided getter.
//
// This method differs from [agent.ForwardToAgent] in that each agent
// forwarding channel is handled with a new connection to the forward
// agent, rather than sharing a single long-lived connection.
//
// Specifically, this is necessary for Windows' named pipe ssh agent
// implementation, as the named pipe connection can be disrupted after
// signature requests. This issue may be resolved directly by the
// [agent] library once https://github.com/golang/go/issues/61383
// is addressed.
//
// The agent getter must be safe to call concurrently.
func ServeChannelRequests(ctx context.Context, client *ssh.Client, getForwardAgent ClientGetter) error {
	channels := client.HandleChannelOpen(channelType)
	if channels == nil {
		return errors.New("agent forwarding channel already open")
	}

	go func() {
		for ch := range channels {
			go func() {
				forwardAgent, err := getForwardAgent()
				if err != nil {
					slog.ErrorContext(ctx, "failed to connect to forwarded agent", "err", err)
					_ = ch.Reject(ssh.ConnectionFailed, ssh.ConnectionFailed.String())
					return
				}
				defer forwardAgent.Close()

				channel, reqs, err := ch.Accept()
				if err != nil {
					return
				}
				defer channel.Close()

				go ssh.DiscardRequests(reqs)
				go io.Copy(io.Discard, channel.Stderr())

				if err := agent.ServeAgent(forwardAgent, channel); err != nil && !errors.Is(err, io.EOF) {
					slog.ErrorContext(ctx, "unexpected error serving forwarded agent", "err", err)
				}
			}()
		}
	}()
	return nil
}

// RequestAgentForwarding sets up agent forwarding for the session.
// ForwardToAgent or ForwardToRemote should be called to route
// the authentication requests.
func RequestAgentForwarding(ctx context.Context, session *tracessh.Session) error {
	ok, err := session.SendRequest(ctx, "auth-agent-req@openssh.com", true, nil)
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.Errorf("agent forwarding request denied")
	}
	return nil
}
