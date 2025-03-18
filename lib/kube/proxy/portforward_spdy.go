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
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/httpstream"
	spdystream "k8s.io/apimachinery/pkg/util/httpstream/spdy"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
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
	idleTimeout        time.Duration
}

func (p portForwardRequest) String() string {
	return fmt.Sprintf("port forward %v/%v -> %v", p.podNamespace, p.podName, p.ports)
}

// portForwardCallback is a callback to be called on every port forward request
type portForwardCallback func(addr string, success bool)

// parsePortString parses a port from a given string.
func parsePortString(pString string) (uint16, error) {
	port, err := strconv.ParseUint(pString, 10, 16)
	if err != nil {
		return 0, trace.BadParameter("unable to parse %q as a port: %v", pString, err)
	}
	if port < 1 {
		return 0, trace.BadParameter("port %q must be > 0", pString)
	}
	return uint16(port), nil
}

// runPortForwardingHTTPStreams upgrades the clients using SPDY protocol.
// It supports multiplexing and HTTP streams and can be used per-request.
func runPortForwardingHTTPStreams(req portForwardRequest) error {
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
			teleport.ComponentKey: teleport.Component(teleport.ComponentProxyKube),
			events.RemoteAddr:     req.httpRequest.RemoteAddr,
		}),
		portForwardRequest:    req,
		sourceConn:            conn,
		streamChan:            streamChan,
		streamPairs:           make(map[string]*httpStreamPair),
		streamCreationTimeout: DefaultStreamCreationTimeout,
		targetConn:            targetConn,
	}
	defer h.Close()

	h.Debugf("Setting port forwarding streaming connection idle timeout to %s.", req.idleTimeout)
	conn.SetIdleTimeout(adjustIdleTimeoutForConn(req.idleTimeout))

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

		_, err := parsePortString(portString)
		if err != nil {
			return trace.Wrap(err)
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

// forwardStreamPair creates a new data and error streams using the same requestID
// received from the client and copies the data between target's data and error and
// client's data and error streams. It blocks until all copy operations complete.
// It does not close the client's data and error streams as they are closed by
// the caller.
func (h *portForwardProxy) forwardStreamPair(p *httpStreamPair, remotePort int64) error {
	// create error stream
	headers := http.Header{}
	port := fmt.Sprintf("%d", remotePort)
	headers.Set(StreamType, StreamTypeError)
	headers.Set(PortHeader, port)
	headers.Set(PortForwardRequestIDHeader, p.requestID)

	// read and write from the error stream
	targetErrorStream, err := h.targetConn.CreateStream(headers)
	h.onPortForward(net.JoinHostPort(h.podName, port), err == nil /* success */)
	if err != nil {
		err := trace.ConnectionProblem(err, "error creating error stream for port %d", remotePort)
		p.sendErr(err)
		return err
	}
	defer func() {
		// on stream close, remove the stream from the connection and close it.
		h.targetConn.RemoveStreams(targetErrorStream)
		targetErrorStream.Close()
	}()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := utils.ProxyConn(h.context, p.errorStream, targetErrorStream); err != nil {
			h.WithError(err).Debugf("Unable to proxy portforward error-stream.")
		}
	}()

	// create data stream
	headers.Set(StreamType, StreamTypeData)
	targetDataStream, err := h.targetConn.CreateStream(headers)
	if err != nil {
		err := trace.ConnectionProblem(err, "error creating forwarding stream for port -> %d: %v", remotePort, err)
		p.sendErr(err)
		return err
	}
	defer func() {
		// on stream close, remove the stream from the connection and close it.
		h.targetConn.RemoveStreams(targetDataStream)
		targetDataStream.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := utils.ProxyConn(h.context, p.dataStream, targetDataStream); err != nil {
			h.WithError(err).Debugf("Unable to proxy portforward data-stream.")
		}
	}()

	h.Debugf("Streams have been created, Waiting for copy to complete.")
	// wait for the copies to complete before returning.
	wg.Wait()
	h.Debugf("Port forwarding pair completed.")
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
func (h *portForwardProxy) monitorStreamPair(p *httpStreamPair) {
	timeC := time.NewTimer(h.streamCreationTimeout)
	defer timeC.Stop()
	select {
	case <-timeC.C:
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
	pair, ok := h.streamPairs[requestID]
	if !ok {
		return
	}
	if h.sourceConn != nil {
		// remove the streams from the connection and close them.
		h.sourceConn.RemoveStreams(pair.dataStream, pair.errorStream)
	}
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
				go h.monitorStreamPair(p)
			}
			if complete, err := p.add(stream); err != nil {
				err := trace.BadParameter("error processing stream for request %s: %v", requestID, err)
				p.sendErr(err)
			} else if complete {
				go h.portForward(p)
			}
		}
	}
}

// portForward handles the port-forwarding for the given stream pair.
// It closes the pair when it is done.
func (h *portForwardProxy) portForward(p *httpStreamPair) {
	defer p.close()

	portString := p.dataStream.Headers().Get(PortHeader)
	port, _ := strconv.ParseInt(portString, 10, 32)

	h.Debugf("Forwarding port %v -> %v.", p.requestID, portString)

	if err := h.forwardStreamPair(p, port); err != nil {
		h.WithError(err).Debugf("Error forwarding port %v -> %v.", p.requestID, portString)
		return
	}
	h.Debugf("Completed forwarding port %v -> %v.", p.requestID, portString)
}

// httpStreamPair represents the error and data streams for a port
// forwarding request.
type httpStreamPair struct {
	lock        sync.Mutex
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

// sendErr writes s to p.errorStream if p.errorStream has been set.
func (p *httpStreamPair) sendErr(err error) {
	if err == nil {
		return
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.errorStream != nil {
		fmt.Fprint(p.errorStream, err.Error())
	}
}

// close closes the data and error streams for this pair.
func (p *httpStreamPair) close() {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.dataStream != nil {
		p.dataStream.Close()
	}
	if p.errorStream != nil {
		p.errorStream.Close()
	}
}
