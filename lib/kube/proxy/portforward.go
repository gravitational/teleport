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

package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/httpstream"
	spdystream "k8s.io/apimachinery/pkg/util/httpstream/spdy"
)

// portForwardRequest is a request that specifies port forwarding
type portForwardRequest struct {
	podNamespace       string
	podName            string
	ports              []string
	httpRequest        *http.Request
	httpResponseWriter http.ResponseWriter
	onPortForward      portForwardCallback
	context            context.Context
	targetDialer       httpstream.Dialer
	pingPeriod         time.Duration
}

func (p portForwardRequest) String() string {
	return fmt.Sprintf("port forward %v/%v -> %v", p.podNamespace, p.podName, p.ports)
}

// portForwardCallback is a callback to be called on every port forward request
type portForwardCallback func(addr string, success bool)

func runPortForwarding(req portForwardRequest) error {
	_, err := httpstream.Handshake(req.httpRequest, req.httpResponseWriter, []string{PortForwardProtocolV1Name})
	if err != nil {
		return trace.Wrap(err)
	}

	targetConn, _, err := req.targetDialer.Dial(PortForwardProtocolV1Name)
	if err != nil {
		return trace.ConnectionProblem(err, "error upgrading connection")
	}
	defer targetConn.Close()

	streamChan := make(chan httpstream.Stream, 1)

	upgrader := spdystream.NewResponseUpgraderWithPings(req.pingPeriod)
	conn := upgrader.UpgradeResponse(req.httpResponseWriter, req.httpRequest, httpStreamReceived(req.context, streamChan))
	if conn == nil {
		return trace.ConnectionProblem(nil, "Unable to upgrade websocket connection")
	}
	defer conn.Close()

	h := &portForwardProxy{
		Entry: log.WithFields(log.Fields{
			trace.Component:   teleport.Component(teleport.ComponentProxyKube),
			events.RemoteAddr: req.httpRequest.RemoteAddr,
		}),
		portForwardRequest:    req,
		sourceConn:            conn,
		streamChan:            streamChan,
		streamPairs:           make(map[string]*httpStreamPair),
		streamCreationTimeout: DefaultStreamCreationTimeout,
		targetConn:            targetConn,
	}
	defer h.Close()
	h.Debugf("Setting port forwarding streaming connection idle timeout to %v", IdleTimeout)
	conn.SetIdleTimeout(IdleTimeout)
	h.run()
	return nil
}

// httpStreamReceived is the httpstream.NewStreamHandler for port
// forward streams. It checks each stream's port and stream type headers,
// rejecting any streams that with missing or invalid values. Each valid
// stream is sent to the streams channel.
func httpStreamReceived(ctx context.Context, streams chan httpstream.Stream) func(httpstream.Stream, <-chan struct{}) error {
	return func(stream httpstream.Stream, replySent <-chan struct{}) error {
		// make sure it has a valid port header
		portString := stream.Headers().Get(PortHeader)
		if len(portString) == 0 {
			return trace.BadParameter("%q header is required", PortHeader)
		}
		port, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return trace.BadParameter("unable to parse %q as a port: %v", portString, err)
		}
		if port < 1 {
			return trace.BadParameter("port %q must be > 0", portString)
		}

		// make sure it has a valid stream type header
		streamType := stream.Headers().Get(StreamType)
		if len(streamType) == 0 {
			return trace.BadParameter("%q header is required", StreamType)
		}
		if streamType != StreamTypeError && streamType != StreamTypeData {
			return trace.BadParameter("invalid stream type %q", streamType)
		}

		select {
		case streams <- stream:
			return nil
		case <-ctx.Done():
			return trace.BadParameter("request has been canceled")
		}
	}
}

// portForwardProxy is capable of processing multiple port forward
// requests over a single httpstream.Connection.
type portForwardProxy struct {
	*log.Entry
	portForwardRequest
	sourceConn            httpstream.Connection
	streamChan            chan httpstream.Stream
	streamPairsLock       sync.RWMutex
	streamPairs           map[string]*httpStreamPair
	streamCreationTimeout time.Duration

	targetConn httpstream.Connection
}

func (h *portForwardProxy) Close() error {
	if h.sourceConn != nil {
		return h.sourceConn.Close()
	}
	return nil
}

func (h *portForwardProxy) forwardStreamPair(p *httpStreamPair, remotePort int64) error {
	// create error stream
	headers := http.Header{}
	headers.Set(StreamType, StreamTypeError)
	headers.Set(PortHeader, fmt.Sprintf("%d", remotePort))
	headers.Set(PortForwardRequestIDHeader, p.requestID)

	// read and write from the error stream
	targetErrorStream, err := h.targetConn.CreateStream(headers)
	if err != nil {
		h.onPortForward(fmt.Sprintf("%v:%v", h.podName, remotePort), false)
		return trace.ConnectionProblem(err, "error creating error stream for port %d", remotePort)
	}
	h.onPortForward(fmt.Sprintf("%v:%v", h.podName, remotePort), true)
	defer targetErrorStream.Close()

	go func() {
		_, err := io.Copy(targetErrorStream, p.errorStream)
		if err != nil && err != io.EOF {
			h.Debugf("Copy stream error: %v.", err)
		}
	}()

	errClose := make(chan struct{})
	go func() {
		defer close(errClose)
		_, err := io.Copy(p.errorStream, targetErrorStream)
		if err != nil && err != io.EOF {
			h.Debugf("Copy stream error: %v.", err)
		}
	}()

	// create data stream
	headers.Set(StreamType, StreamTypeData)
	dataStream, err := h.targetConn.CreateStream(headers)
	if err != nil {
		return trace.ConnectionProblem(err, "error creating forwarding stream for port -> %d: %v", remotePort, err)
	}
	defer dataStream.Close()

	localError := make(chan struct{})
	remoteDone := make(chan struct{})

	go func() {
		// inform the select below that the remote copy is done
		defer close(remoteDone)
		// Copy from the remote side to the local port.
		if _, err := io.Copy(p.dataStream, dataStream); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Error(fmt.Errorf("error copying from remote stream to local connection: %v", err))
		}
	}()

	go func() {
		// inform server we're not sending any more data after copy unblocks
		defer dataStream.Close()

		// Copy from the local port to the target side.
		if _, err := io.Copy(dataStream, p.dataStream); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
			h.Warningf("Error copying from local connection to remote stream: %v.", err)
			// break out of the select below without waiting for the other copy to finish
			close(localError)
		}
	}()

	h.Debugf("Streams have been created, Waiting for copy to complete.")

	// wait for either a local->remote error or for copying from remote->local to finish
	select {
	case <-remoteDone:
	case <-localError:
	case <-h.context.Done():
		h.Debugf("Context is closing, cleaning up.")
	}

	// always expect something on errorChan (it may be nil)
	select {
	case <-errClose:
	case <-h.context.Done():
		h.Debugf("Context is closing, cleaning up.")
	}
	h.Infof("Port forwarding pair completed.")
	return nil
}

// getStreamPair returns a httpStreamPair for requestID. This creates a
// new pair if one does not yet exist for the requestID. The returned bool is
// true if the pair was created.
func (h *portForwardProxy) getStreamPair(requestID string) (*httpStreamPair, bool) {
	h.streamPairsLock.Lock()
	defer h.streamPairsLock.Unlock()

	if p, ok := h.streamPairs[requestID]; ok {
		log.Debugf("Request %s, found existing stream pair", requestID)
		return p, false
	}

	h.Debugf("Request %s, creating new stream pair.", requestID)

	p := newPortForwardPair(requestID)
	h.streamPairs[requestID] = p

	return p, true
}

// monitorStreamPair waits for the pair to receive both its error and data
// streams, or for the timeout to expire (whichever happens first), and then
// removes the pair.
func (h *portForwardProxy) monitorStreamPair(p *httpStreamPair, timeout <-chan time.Time) {
	select {
	case <-timeout:
		h.Errorf("Request %s, timed out waiting for streams.", p.requestID)
	case <-p.complete:
		h.Debugf("Request %s, successfully received error and data streams.", p.requestID)
	}
	h.removeStreamPair(p.requestID)
}

// removeStreamPair removes the stream pair identified by requestID from streamPairs.
func (h *portForwardProxy) removeStreamPair(requestID string) {
	h.streamPairsLock.Lock()
	defer h.streamPairsLock.Unlock()

	delete(h.streamPairs, requestID)
}

// requestID returns the request id for stream.
func (h *portForwardProxy) requestID(stream httpstream.Stream) (string, error) {
	requestID := stream.Headers().Get(PortForwardRequestIDHeader)
	if len(requestID) == 0 {
		return "", trace.BadParameter("port forwarding is not supported")
	}
	return requestID, nil
}

// run is the main loop for the portForwardProxy. It processes new
// streams, invoking portForward for each complete stream pair. The loop exits
// when the httpstream.Connection is closed.
func (h *portForwardProxy) run() {
	h.Debugf("Waiting for port forward streams.")
	for {
		select {
		case <-h.context.Done():
			h.Debugf("Context is closing, returning.")
			return
		case <-h.sourceConn.CloseChan():
			h.Debugf("Upgraded connection closed.")
			return
		case stream := <-h.streamChan:
			requestID, err := h.requestID(stream)
			if err != nil {
				h.Warningf("Failed to parse request id: %v.", err)
				return
			}
			streamType := stream.Headers().Get(StreamType)
			h.Debugf("Received new stream %v of type %v.", requestID, streamType)

			p, created := h.getStreamPair(requestID)
			if created {
				go h.monitorStreamPair(p, time.After(h.streamCreationTimeout))
			}
			if complete, err := p.add(stream); err != nil {
				msg := fmt.Sprintf("error processing stream for request %s: %v", requestID, err)
				p.printError(msg)
			} else if complete {
				go h.portForward(p)
			}
		}
	}
}

// portForward invokes the portForwardProxy's forwarder.PortForward
// function for the given stream pair.
func (h *portForwardProxy) portForward(p *httpStreamPair) {
	defer p.dataStream.Close()
	defer p.errorStream.Close()

	portString := p.dataStream.Headers().Get(PortHeader)
	port, _ := strconv.ParseInt(portString, 10, 32)

	h.Debugf("Forwarding port %v -> %v.", p.requestID, portString)
	err := h.forwardStreamPair(p, port)
	h.Debugf("Completed forwarding port %v -> %v.", p.requestID, portString)

	if err != nil {
		msg := fmt.Errorf("error forwarding port %d to pod %s: %v", port, h.podName, err)
		fmt.Fprint(p.errorStream, msg.Error())
	}
}

// httpStreamPair represents the error and data streams for a port
// forwarding request.
type httpStreamPair struct {
	lock        sync.RWMutex
	requestID   string
	dataStream  httpstream.Stream
	errorStream httpstream.Stream
	complete    chan struct{}
}

// newPortForwardPair creates a new httpStreamPair.
func newPortForwardPair(requestID string) *httpStreamPair {
	return &httpStreamPair{
		requestID: requestID,
		complete:  make(chan struct{}),
	}
}

// add adds the stream to the httpStreamPair. If the pair already
// contains a stream for the new stream's type, an error is returned. add
// returns true if both the data and error streams for this pair have been
// received.
func (p *httpStreamPair) add(stream httpstream.Stream) (bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	switch stream.Headers().Get(StreamType) {
	case StreamTypeError:
		if p.errorStream != nil {
			return false, trace.BadParameter("error stream already assigned")
		}
		p.errorStream = stream
	case StreamTypeData:
		if p.dataStream != nil {
			return false, trace.BadParameter("data stream already assigned")
		}
		p.dataStream = stream
	}

	complete := p.errorStream != nil && p.dataStream != nil
	if complete {
		close(p.complete)
	}
	return complete, nil
}

// printError writes s to p.errorStream if p.errorStream has been set.
func (p *httpStreamPair) printError(s string) {
	p.lock.RLock()
	defer p.lock.RUnlock()
	if p.errorStream != nil {
		fmt.Fprint(p.errorStream, s)
	}
}
