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
	targetConn, _, err := req.targetDialer.Dial(PortForwardProtocolV1Name)
	if err != nil {
		return trace.Wrap(err)
	}
	defer targetConn.Close()

	_, err = httpstream.Handshake(req.httpRequest, req.httpResponseWriter, []string{PortForwardProtocolV1Name})
	if err != nil {
		return trace.Wrap(err)
	}
	streamChan := make(chan streamAndReply, 1)
	// streamCtx is canceled when the proxy loop stops accepting streams, see portForwardProxy.run.
	streamCtx, stopStreams := context.WithCancel(req.context)
	defer stopStreams()

	upgrader := spdystream.NewResponseUpgraderWithPings(req.pingPeriod)
	conn := upgrader.UpgradeResponse(req.httpResponseWriter, req.httpRequest, httpStreamReceived(streamCtx, streamChan))
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
		stopStreams:           stopStreams,
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
func httpStreamReceived(ctx context.Context, streams chan streamAndReply) func(httpstream.Stream, <-chan struct{}) error {
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

		// Reply before handing the stream over. The caller (httpstream/spdy newSpdyStream)
		// only replies after this handler returns and ignores a failed reply. spdystream
		// blocks every Write/Close on a received stream until the reply has been sent,
		// with no escape on connection close. If the client disconnects in between, the
		// reply fails and any later Write/Close on the stream parks forever. Replying here
		// means only streams that were acknowledged ever reach the proxy loop, and a failed
		// reply rejects the stream (which resets it) instead. The later reply is a no-op.
		// Realistically this should really be addressed by httpstream/spdy itself,
		// but this is a workaround for now.
		if r, ok := stream.(streamReplier); ok {
			if err := r.SendReply(http.Header{}, false); err != nil {
				return trace.ConnectionProblem(err, "unable to reply to stream")
			}
		}

		// select picks randomly among ready cases, check ctx defensively here.
		// This is kept buffered to avoid blocking the frame worker.
		if err := ctx.Err(); err != nil {
			return trace.BadParameter("request has been canceled")
		}

		select {
		case streams <- streamAndReply{Stream: stream, replySent: replySent}:
			return nil
		case <-ctx.Done():
			return trace.BadParameter("request has been canceled")
		}
	}
}

// streamReplier is implemented by the *spdystream.Stream that
// k8s.io/apimachinery/pkg/util/httpstream/spdy hands to a stream handler.
type streamReplier interface {
	SendReply(headers http.Header, fin bool) error
}

// portForwardProxy is capable of processing multiple port forward
// requests over a single httpstream.Connection.
type portForwardProxy struct {
	*log.Entry
	portForwardRequest
	sourceConn httpstream.Connection
	streamChan chan streamAndReply
	// stopStreams cancels the context used by httpStreamReceived, so stream handlers
	// blocked handing a stream to the loop return once the loop stops reading streamChan.
	stopStreams           context.CancelFunc
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
// client's data and error streams. It returns once the data stream is done and
// the error stream has either been closed by the target or reset after
// errStreamGracePeriod.
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
		// Unread data frames on the client data stream can block the frames, reset to unblock.
		_ = p.dataStream.Reset()
		return err
	}
	defer h.targetConn.RemoveStreams(targetErrorStream)

	// We never write to the error stream. Note that Close only half-closes the
	// stream, it does not unblock the Read, only the target closing its
	// side or a Reset does.
	if err := targetErrorStream.Close(); err != nil {
		h.WithError(err).Debug("Unable to close target error stream")
	}

	errDone := make(chan struct{})
	go func() {
		defer close(errDone)
		// Enables error propagation from Kube API server to kubectl client.
		if _, err := io.Copy(p.errorStream, targetErrorStream); err != nil {
			h.WithError(err).Debug("Unable to proxy portforward error-stream")
		}
	}()

	// create data stream
	headers.Set(StreamType, StreamTypeData)
	targetDataStream, err := h.targetConn.CreateStream(headers)
	if err != nil {
		// Unblock the Read() and wait for the error stream copy before sending our error to the client.
		_ = targetErrorStream.Reset()
		<-errDone
		err := trace.ConnectionProblem(err, "error creating forwarding stream for port -> %d: %v", remotePort, err)
		p.sendErr(err)
		// Unread data frames on the client data stream can block the frames, reset to unblock.
		_ = p.dataStream.Reset()
		return err
	}
	defer h.targetConn.RemoveStreams(targetDataStream)

	h.Debug("Streams have been created, waiting for copy to complete")

	// The target data stream must be reset before waiting on the error stream,
	// otherwise unread data frames can block delivery of the error stream close.
	h.proxyDataStream(p.dataStream, targetDataStream)
	h.waitErrorStream(targetErrorStream, errDone)

	h.Debug("Port forwarding pair completed")
	return nil
}

// proxyDataStream copies data between the client and target data streams until
// the target is done sending, the client copy fails, the client connection
// closes, or the request context is done, then tears both streams down and
// waits for the copies to return.
//
// The two directions are not symmetric:
//
//   - client -> target ending cleanly is treated as a half-close. spdystream
//     returns io.EOF for a FIN, an RST and a dropped connection alike; the
//     dropped connection is caught by sourceConn.CloseChan, while an RST is
//     handled like a FIN and lasts until the target closes its side.
//   - client -> target ending with an error (in practice a failed write to the
//     target), the target -> client copy ending, the client connection closing,
//     or the context being done ends the pair.
//
// Once the pair ends both streams are closed and reset: Close sends a FIN
// after any data already written, while Reset discards unread data and
// unblocks the pending Read on that stream.
//
// IMPORTANT: Unread data frames left on a SPDY stream stall
// frame processing for the whole connection, which in turn prevents the error
// stream from ever completing.
//
// This mirrors client-go's port forwarder, which waits for the remote copy or
// a local copy error:
// https://github.com/kubernetes/client-go/blob/master/tools/portforward/portforward.go
func (h *portForwardProxy) proxyDataStream(client, target httpstream.Stream) {
	remoteDone := make(chan struct{})
	localDone := make(chan struct{})
	// localErr is closed only when the client -> target copy fails. A clean
	// end of that copy is a half-close and does not end the pair.
	localErr := make(chan struct{})

	go func() {
		defer close(remoteDone)
		if _, err := io.Copy(client, target); err != nil && !utils.IsOKNetworkError(err) {
			h.WithError(err).Debug("Unable to copy from target data stream to client")
		}
	}()
	go func() {
		defer close(localDone)
		_, err := io.Copy(target, client)
		if err == nil {
			// The client half-closed. Pass the FIN on and keep waiting for the
			// target to finish sending.
			if err := target.Close(); err != nil {
				h.WithError(err).Debug("Unable to half-close target data stream")
			}
			return
		}
		if !utils.IsOKNetworkError(err) {
			h.WithError(err).Debug("Unable to copy from client data stream to target")
		}
		close(localErr)
	}()

	// This allows unit testing without source connection.
	var sourceClosed <-chan bool
	if h.sourceConn != nil {
		sourceClosed = h.sourceConn.CloseChan()
	}

	select {
	case <-remoteDone:
	case <-localErr:
	case <-sourceClosed:
	case <-h.context.Done():
	}

	_ = target.Close()
	_ = target.Reset()

	// Tell the client no more data is coming, then reset so the pending client
	// Read returns. Teardown must not depend on the client closing or resetting
	// its side, otherwise a client that keeps the stream open pins this pair
	// (and the target streams) until the connection goes away.
	_ = client.Close()
	_ = client.Reset()

	<-remoteDone
	<-localDone
}

// waitErrorStream waits for the target error stream copy to finish. If the
// target has not closed the error stream within errStreamGracePeriod, or the
// request context is done, or the underlying source connection is closed, the stream is reset so the pending Read returns.
func (h *portForwardProxy) waitErrorStream(target httpstream.Stream, done <-chan struct{}) {
	const errStreamGracePeriod = 5 * time.Second

	var sourceClosed <-chan bool
	if h.sourceConn != nil {
		sourceClosed = h.sourceConn.CloseChan()
	}

	select {
	case <-done:
		return
	case <-time.After(errStreamGracePeriod):
		h.WithField("grace_period", errStreamGracePeriod).Debug("Target error stream not closed within grace period, resetting")
	case <-h.context.Done():
	case <-sourceClosed:
		// The client connection closed.
	}

	_ = target.Reset()
	<-done
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
	h.Debugf("Waiting for port forward streams")
	var wg sync.WaitGroup
	defer func() {
		// Nothing reads streamChan past this point. Unblock stream handlers still trying
		// to hand over a stream before waiting on the forwarders.
		// spdystream only closes the source streams after every frame worker returns.
		if h.stopStreams != nil {
			h.stopStreams()
		}
		wg.Wait()
	}()
	for {
		select {
		case <-h.context.Done():
			h.Debugf("Context is closing, returning.")
			return
		case <-h.sourceConn.CloseChan():
			h.Debugf("Upgraded connection closed.")
			return
		case <-h.targetConn.CloseChan():
			h.Debugf("Target connection closed")
			return
		case s := <-h.streamChan:
			// k8s.io/streaming only registers the stream with the connection after the
			// handler returns. Wait for the reply so that removeStreamPair cannot run
			// before the registration and leave the stream behind in the connection.
			select {
			case <-s.replySent:
			case <-h.context.Done():
				return
			case <-h.sourceConn.CloseChan():
				return
			}
			stream := s.Stream

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
				wg.Add(1)
				go func() {
					defer wg.Done()
					h.portForward(p)
				}()
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
