/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Origin: https://github.com/kubernetes/kubernetes/blob/d5fdf3135e7c99e5f81e67986ae930f6a2ffb047/pkg/kubelet/cri/streaming/remotecommand/websocket.go

package proxy

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/httpstream/wsstream"
	"k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/apiserver/pkg/endpoints/responsewriter"
	"k8s.io/klog/v2"
)

const (
	preV4BinaryWebsocketProtocol = wsstream.ChannelWebSocketProtocol
	preV4Base64WebsocketProtocol = wsstream.Base64ChannelWebSocketProtocol
	v4BinaryWebsocketProtocol    = "v4." + wsstream.ChannelWebSocketProtocol
	v4Base64WebsocketProtocol    = "v4." + wsstream.Base64ChannelWebSocketProtocol
	v5BinaryWebsocketProtocol    = remotecommand.StreamProtocolV5Name
)

func init() {
	// Replace the default logger from Kubernetes klog package with one that does not log anything.
	// This is required to suppress log messages from wsstream when forcing the connection to close.
	// Error logs are emitted because `wsstream` does not properly close websocket connections -
	// instead of closing only the server side it closes the full connection while the server is
	// still waiting for the client to close it.
	// Examples of logs emitted by bad behavior are:
	// - Use of closed network connection
	// - Error on socket receive: read tcp 192.168.1.236:3027->192.168.1.236:57842: use of closed
	//   network connection

	// Go init running order guarantees that the klog package is initialized before this package.
	klog.SetLoggerWithOptions(logr.Discard())
}

// createChannels returns the standard channel types for a shell connection (STDIN 0, STDOUT 1, STDERR 2)
// along with the approximate duplex value. It also creates the error (3) and resize (4) channels.
func createChannels(req remoteCommandRequest) []wsstream.ChannelType {
	// open the requested channels, and always open the error channel
	channels := make([]wsstream.ChannelType, 5)
	channels[remotecommand.StreamStdIn] = readChannel(req.stdin)
	channels[remotecommand.StreamStdOut] = writeChannel(req.stdout)
	channels[remotecommand.StreamStdErr] = writeChannel(req.stderr)
	channels[remotecommand.StreamErr] = wsstream.WriteChannel
	channels[remotecommand.StreamResize] = wsstream.ReadChannel
	return channels
}

// readChannel returns wsstream.ReadChannel if real is true, or wsstream.IgnoreChannel.
func readChannel(real bool) wsstream.ChannelType {
	if real {
		return wsstream.ReadChannel
	}
	return wsstream.IgnoreChannel
}

// writeChannel returns wsstream.WriteChannel if real is true, or wsstream.IgnoreChannel.
func writeChannel(real bool) wsstream.ChannelType {
	if real {
		return wsstream.WriteChannel
	}
	return wsstream.IgnoreChannel
}

// createWebSocketStreams returns a context containing the websocket connection and
// streams needed to perform an exec or an attach.
func createWebSocketStreams(req remoteCommandRequest) (*remoteCommandProxy, error) {
	channels := createChannels(req)
	conn := wsstream.NewConn(map[string]wsstream.ChannelProtocolConfig{
		"": {
			Binary:   true,
			Channels: channels,
		},
		preV4BinaryWebsocketProtocol: {
			Binary:   true,
			Channels: channels,
		},
		preV4Base64WebsocketProtocol: {
			Binary:   false,
			Channels: channels,
		},
		v4BinaryWebsocketProtocol: {
			Binary:   true,
			Channels: channels,
		},
		v4Base64WebsocketProtocol: {
			Binary:   false,
			Channels: channels,
		},
		v5BinaryWebsocketProtocol: {
			Binary:   true,
			Channels: channels,
		},
	})

	conn.SetIdleTimeout(adjustIdleTimeoutForConn(req.idleTimeout))

	negotiatedProtocol, streams, err := conn.Open(
		responsewriter.GetOriginal(req.httpResponseWriter),
		req.httpRequest,
	)
	if err != nil {
		return nil, trace.Wrap(err, "unable to upgrade websocket connection")
	}

	// Send an empty message to the lowest writable channel to notify the client the connection is established
	switch {
	case req.stdout:
		streams[remotecommand.StreamStdOut].Write([]byte{})
	case req.stderr:
		streams[remotecommand.StreamStdErr].Write([]byte{})
	default:
		streams[streamErr].Write([]byte{})
	}

	proxy := &remoteCommandProxy{
		conn:         conn,
		stdinStream:  streams[remotecommand.StreamStdIn],
		stdoutStream: streams[remotecommand.StreamStdOut],
		stderrStream: streams[remotecommand.StreamStdErr],
		tty:          req.tty,
		resizeStream: streams[remotecommand.StreamResize],
	}

	// When stdin, stdout or stderr are not enabled, websocket creates a io.Pipe
	// for them so they are not nil.
	// Since we need to forward to another k8s server (Teleport or real k8s API),
	// we must disabled the readers, otherwise the SPDY executor will wait for
	// read/write into the streams and will hang.
	if !req.stdin {
		proxy.stdinStream = nil
	}
	if !req.stdout {
		proxy.stdoutStream = nil
	}
	if !req.stderr {
		proxy.stderrStream = nil
	}

	switch negotiatedProtocol {
	case v5BinaryWebsocketProtocol, v4BinaryWebsocketProtocol, v4Base64WebsocketProtocol:
		proxy.writeStatus = v4WriteStatusFunc(streams[remotecommand.StreamErr])
	default:
		proxy.writeStatus = v1WriteStatusFunc(streams[remotecommand.StreamErr])
	}

	return proxy, nil
}

// adjustIdleTimeoutForConn adjusts the idle timeout for the connection
// to be 5 seconds longer than the requested idle timeout.
// This is done to prevent the connection from being closed by the server
// before the connection monitor has a chance to close it and write the
// status code.
// If the idle timeout is 0, this function returns 0 because it means the
// connection will never be closed by the server due to idleness.
func adjustIdleTimeoutForConn(idleTimeout time.Duration) time.Duration {
	// If the idle timeout is 0, we don't need to adjust it because it
	// means the connection will never be closed by the server due to idleness.
	if idleTimeout != 0 {
		idleTimeout += 5 * time.Second
	}
	return idleTimeout
}
