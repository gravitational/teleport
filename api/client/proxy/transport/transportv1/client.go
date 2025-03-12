// Copyright 2023 Gravitational, Inc
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

package transportv1

import (
	"context"
	"net"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc/peer"

	transportv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/transport/v1"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
)

// Client is a wrapper around a [transportv1.TransportServiceClient] that
// hides the implementation details of establishing connections
// over gRPC streams.
type Client struct {
	clt transportv1pb.TransportServiceClient
}

// NewClient constructs a Client that operates on the provided
// [transportv1pb.TransportServiceClient]. An error is returned if the client
// provided is nil.
func NewClient(client transportv1pb.TransportServiceClient) (*Client, error) {
	if client == nil {
		return nil, trace.BadParameter("parameter client required")
	}

	return &Client{clt: client}, nil
}

// ClusterDetails retrieves the cluster details as observed by the Teleport Proxy
// that the Client is connected to.
func (c *Client) ClusterDetails(ctx context.Context) (*transportv1pb.ClusterDetails, error) {
	resp, err := c.clt.GetClusterDetails(ctx, &transportv1pb.GetClusterDetailsRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp.Details, nil
}

// DialCluster establishes a connection to the provided cluster. The provided
// src address will be used as the LocalAddr of the returned [net.Conn].
func (c *Client) DialCluster(ctx context.Context, cluster string, src net.Addr) (net.Conn, error) {
	// we do this rather than using context.Background to inherit any OTEL data
	// from the dial context
	connCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, cancel)
	defer stop()

	stream, err := c.clt.ProxyCluster(connCtx)
	if err != nil {
		cancel()
		return nil, trace.Wrap(err, "unable to establish proxy stream")
	}

	if err := stream.Send(&transportv1pb.ProxyClusterRequest{Cluster: cluster}); err != nil {
		cancel()
		return nil, trace.Wrap(err, "failed to send cluster request")
	}

	if !stop() {
		cancel()
		return nil, trace.Wrap(connCtx.Err(), "unable to establish proxy stream")
	}

	streamRW, err := streamutils.NewReadWriter(clusterStream{stream: stream, cancel: cancel})
	if err != nil {
		cancel()
		return nil, trace.Wrap(err, "unable to create stream reader")
	}

	p, ok := peer.FromContext(stream.Context())
	if !ok {
		streamRW.Close()
		return nil, trace.BadParameter("unable to retrieve peer information")
	}

	return streamutils.NewConn(streamRW, src, p.Addr), nil
}

// clusterStream implements the [streamutils.Source] interface
// for a [transportv1pb.TransportService_ProxyClusterClient].
type clusterStream struct {
	stream transportv1pb.TransportService_ProxyClusterClient
	cancel context.CancelFunc
}

func (c clusterStream) Recv() ([]byte, error) {
	req, err := c.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Frame == nil {
		return nil, trace.BadParameter("received invalid frame")
	}

	return req.Frame.Payload, nil
}

func (c clusterStream) Send(frame []byte) error {
	return trace.Wrap(c.stream.Send(&transportv1pb.ProxyClusterRequest{Frame: &transportv1pb.Frame{Payload: frame}}))
}

func (c clusterStream) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

// DialHost establishes a connection to the instance in the provided cluster that matches
// the hostport. If a keyring is provided then it will be forwarded to the remote instance.
// The src address will be used as the LocalAddr of the returned [net.Conn].
func (c *Client) DialHost(ctx context.Context, hostport, cluster, loginName string, src net.Addr, keyring agent.ExtendedAgent) (net.Conn, *transportv1pb.ClusterDetails, error) {
	ctx, cancel := context.WithCancel(ctx)
	stream, err := c.clt.ProxySSH(ctx)
	if err != nil {
		cancel()
		return nil, nil, trace.Wrap(err, "unable to establish proxy stream")
	}

	if err := stream.Send(&transportv1pb.ProxySSHRequest{DialTarget: &transportv1pb.TargetHost{
		HostPort: hostport,
		Cluster:  cluster,

		LoginName: loginName,
	}}); err != nil {
		cancel()
		return nil, nil, trace.Wrap(err, "failed to send dial target request")
	}

	resp, err := stream.Recv()
	if err != nil {
		cancel()
		return nil, nil, trace.Wrap(err, "failed to receive cluster details response")
	}

	// create streams for ssh and agent protocol
	sshStream, agentStream := newSSHStreams(stream, cancel)

	// create a reader writer for agent protocol
	agentRW, err := streamutils.NewReadWriter(agentStream)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// create a reader writer for SSH protocol
	sshRW, err := streamutils.NewReadWriter(sshStream)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	sshConn := streamutils.NewConn(sshRW, src, addr(hostport))

	// multiplex the frames to the correct handlers
	var serveOnce sync.Once
	go func() {
		defer func() {
			// closing the agentRW will terminate the agent.ServeAgent goroutine
			agentRW.Close()
			// closing the connection will close sshRW and end the connection for
			// the user
			sshConn.Close()
		}()

		for {
			req, err := stream.Recv()
			if err != nil {
				sshStream.errorC <- trace.Wrap(err)
				agentStream.errorC <- trace.Wrap(err)
				return
			}

			switch frame := req.Frame.(type) {
			case *transportv1pb.ProxySSHResponse_Ssh:
				sshStream.incomingC <- frame.Ssh.Payload
			case *transportv1pb.ProxySSHResponse_Agent:
				if keyring == nil {
					continue
				}

				// start serving the agent only if the upstream
				// service attempts to interact with it
				serveOnce.Do(func() {
					go agent.ServeAgent(keyring, agentRW)
				})

				agentStream.incomingC <- frame.Agent.Payload
			default:
				continue
			}
		}
	}()

	return sshConn, resp.Details, nil
}

type addr string

func (a addr) Network() string {
	return "tcp"
}

func (a addr) String() string {
	return string(a)
}

// sshStream implements the [streamutils.Source] interface
// for a [transportv1pb.TransportService_ProxySSHClient]. Instead of
// reading directly from the stream reads are from an incoming
// channel that is fed by the multiplexer.
type sshStream struct {
	incomingC chan []byte
	errorC    chan error
	requestFn func(payload []byte) *transportv1pb.ProxySSHRequest
	closedC   chan struct{}
	wLock     *sync.Mutex
	stream    transportv1pb.TransportService_ProxySSHClient
	cancel    context.CancelFunc
}

func newSSHStreams(stream transportv1pb.TransportService_ProxySSHClient, cancel context.CancelFunc) (ssh *sshStream, agent *sshStream) {
	wLock := &sync.Mutex{}
	closedC := make(chan struct{})

	ssh = &sshStream{
		incomingC: make(chan []byte, 10),
		errorC:    make(chan error, 1),
		stream:    stream,
		requestFn: func(payload []byte) *transportv1pb.ProxySSHRequest {
			return &transportv1pb.ProxySSHRequest{Frame: &transportv1pb.ProxySSHRequest_Ssh{Ssh: &transportv1pb.Frame{Payload: payload}}}
		},
		wLock:   wLock,
		closedC: closedC,
		cancel:  cancel,
	}

	agent = &sshStream{
		incomingC: make(chan []byte, 10),
		errorC:    make(chan error, 1),
		stream:    stream,
		requestFn: func(payload []byte) *transportv1pb.ProxySSHRequest {
			return &transportv1pb.ProxySSHRequest{Frame: &transportv1pb.ProxySSHRequest_Agent{Agent: &transportv1pb.Frame{Payload: payload}}}
		},
		wLock:   wLock,
		closedC: closedC,
		cancel:  cancel,
	}

	return ssh, agent
}

func (s *sshStream) Recv() ([]byte, error) {
	select {
	case err := <-s.errorC:
		return nil, trace.Wrap(err)
	case frame := <-s.incomingC:
		return frame, nil
	}
}

func (s *sshStream) Send(frame []byte) error {
	// grab lock to prevent any other sends from occurring
	s.wLock.Lock()
	defer s.wLock.Unlock()

	// only Send if the stream hasn't already been closed
	select {
	case <-s.closedC:
		return nil
	default:
		return trace.Wrap(s.stream.Send(s.requestFn(frame)))
	}
}

func (s *sshStream) Close() error {
	s.cancel()
	// grab lock to prevent any sends from occurring
	s.wLock.Lock()
	defer s.wLock.Unlock()

	// only CloseSend if the stream hasn't already been closed
	select {
	case <-s.closedC:
		return nil
	default:
		close(s.closedC)
		return trace.Wrap(s.stream.CloseSend())
	}
}
