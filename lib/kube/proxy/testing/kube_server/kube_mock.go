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

package kubeserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	v1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/httpstream"
	spdystream "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/apimachinery/pkg/util/httpstream/wsstream"
	apiremotecommand "k8s.io/apimachinery/pkg/util/remotecommand"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/endpoints/responsewriter"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// The SPDY subprotocol "v4.channel.k8s.io" is used for remote command
	// attachment/execution. It is the 4th version of the subprotocol and
	// adds support for exit codes.
	StreamProtocolV4Name = "v4.channel.k8s.io"

	// DefaultStreamCreationTimeout
	DefaultStreamCreationTimeout = 30 * time.Second

	IdleTimeout = 15 * time.Minute
	// Name of header that specifies stream type
	StreamType = "streamType"
	// Value for streamType header for stdin stream
	StreamTypeStdin = "stdin"
	// Value for streamType header for stdout stream
	StreamTypeStdout = "stdout"
	// Value for streamType header for stderr stream
	StreamTypeStderr = "stderr"
	// Value for streamType header for data stream
	StreamTypeData = "data"
	// Value for streamType header for error stream
	StreamTypeError = "error"
	// Value for streamType header for terminal resize stream
	StreamTypeResize = "resize"

	preV4BinaryWebsocketProtocol = wsstream.ChannelWebSocketProtocol
	preV4Base64WebsocketProtocol = wsstream.Base64ChannelWebSocketProtocol
	v4BinaryWebsocketProtocol    = "v4." + wsstream.ChannelWebSocketProtocol
	v4Base64WebsocketProtocol    = "v4." + wsstream.Base64ChannelWebSocketProtocol
	v5BinaryWebsocketProtocol    = "v5." + wsstream.ChannelWebSocketProtocol

	// CloseStreamMessage is an expected keyword if stdin is enable and the
	// underlying protocol does not support half closed streams.
	// It is only required for websockets.
	CloseStreamMessage = "\r\nexit_message\r\n"

	// portForwardProtocolV1Name is the subprotocol "portforward.k8s.io" is used for port forwarding
	portForwardProtocolV1Name = "portforward.k8s.io"
	// portHeader is the "container" port to forward
	portHeader = "port"

	// PortForwardPayload is the message that dummy portforward handler writes
	// into the connection before terminating the portforward connection.
	PortForwardPayload = "Portforward handler message"
)

// Option is a functional option for KubeMockServer
type Option func(*KubeMockServer)

// WithGetPodError sets the error to be returned by the GetPod call
func WithGetPodError(status metav1.Status) Option {
	return func(s *KubeMockServer) {
		s.getPodError = &status
	}
}

// WithExecError sets the error to be returned by the Exec call
func WithExecError(status metav1.Status) Option {
	return func(s *KubeMockServer) {
		s.execPodError = &status
	}
}

// WithVersion sets the version of the server
func WithVersion(version *apimachineryversion.Info) Option {
	return func(s *KubeMockServer) {
		s.version = version
	}
}

type deletedResource struct {
	requestID string
	kind      string
}

// KubeExecRequests keeps track of the number of exec requests
type KubeExecRequests struct {
	// SPDY is the number of SPDY exec requests
	SPDY atomic.Int32
	// Websocket is the number of Websocket exec requests
	Websocket atomic.Int32
}

type KubeMockServer struct {
	router           *httprouter.Router
	log              *log.Entry
	server           *httptest.Server
	TLS              *tls.Config
	URL              string
	Address          string
	CA               []byte
	deletedResources map[deletedResource][]string
	getPodError      *metav1.Status
	execPodError     *metav1.Status
	mu               sync.Mutex
	version          *apimachineryversion.Info
	KubeExecRequests
}

// NewKubeAPIMock creates Kubernetes API server for handling exec calls.
// For now it just supports exec via SPDY protocol and returns the following content into the available streams:
// {containerName}\n
// {stdinDump}
// The output returns the container followed by a dump of the data received from stdin.
// More endpoints can be configured
// TODO(tigrato): add support for other endpoints
func NewKubeAPIMock(opts ...Option) (*KubeMockServer, error) {
	s := &KubeMockServer{
		router:           httprouter.New(),
		log:              log.NewEntry(log.New()),
		deletedResources: make(map[deletedResource][]string),
		version: &apimachineryversion.Info{
			Major: "1",
			Minor: "20",
		},
	}

	for _, o := range opts {
		o(s)
	}

	s.setup()
	if err := http2.ConfigureServer(s.server.Config, &http2.Server{}); err != nil {
		return nil, err
	}
	s.server.StartTLS()
	s.TLS = s.server.TLS
	s.Address = strings.TrimPrefix(s.server.URL, "https://")
	s.URL = s.server.URL
	return s, nil
}

func (s *KubeMockServer) setup() {
	s.router.UseRawPath = true
	s.router.POST("/api/:ver/namespaces/:namespace/pods/:name/exec", s.withWriter(s.exec))
	s.router.GET("/api/:ver/namespaces/:namespace/pods/:name/exec", s.withWriter(s.exec))
	s.router.GET("/api/:ver/namespaces/:namespace/pods/:name/portforward", s.withWriter(s.portforward))
	s.router.POST("/api/:ver/namespaces/:namespace/pods/:name/portforward", s.withWriter(s.portforward))

	s.router.GET("/apis/rbac.authorization.k8s.io/:ver/clusterroles", s.withWriter(s.listClusterRoles))
	s.router.GET("/apis/rbac.authorization.k8s.io/:ver/clusterroles/:name", s.withWriter(s.getClusterRole))
	s.router.DELETE("/apis/rbac.authorization.k8s.io/:ver/clusterroles/:name", s.withWriter(s.deleteClusterRole))

	s.router.GET("/api/:ver/namespaces/:namespace/pods", s.withWriter(s.listPods))
	s.router.GET("/api/:ver/pods", s.withWriter(s.listPods))
	s.router.GET("/api/:ver/namespaces/:namespace/pods/:name", s.withWriter(s.getPod))
	s.router.DELETE("/api/:ver/namespaces/:namespace/pods/:name", s.withWriter(s.deletePod))

	s.router.GET("/api/:ver/namespaces/:namespace/secrets", s.withWriter(s.listSecrets))
	s.router.GET("/api/:ver/secrets", s.withWriter(s.listSecrets))
	s.router.GET("/api/:ver/namespaces/:namespace/secrets/:name", s.withWriter(s.getSecret))
	s.router.DELETE("/api/:ver/namespaces/:namespace/secrets/:name", s.withWriter(s.deleteSecret))

	s.router.POST("/apis/authorization.k8s.io/v1/selfsubjectaccessreviews", s.withWriter(s.selfSubjectAccessReviews))

	s.router.GET("/apis/resources.teleport.dev/v6/namespaces/:namespace/teleportroles", s.withWriter(s.listTeleportRoles))
	s.router.GET("/apis/resources.teleport.dev/v6/teleportroles", s.withWriter(s.listTeleportRoles))
	s.router.GET("/apis/resources.teleport.dev/v6/namespaces/:namespace/teleportroles/:name", s.withWriter(s.getTeleportRole))
	s.router.DELETE("/apis/resources.teleport.dev/v6/namespaces/:namespace/teleportroles/:name", s.withWriter(s.deleteTeleportRole))

	s.router.GET("/version", s.withWriter(s.versionEndpoint))

	for _, endpoint := range []string{"/api", "/api/:ver", "/apis", "/apis/resources.teleport.dev/v6"} {
		s.router.GET(endpoint, s.withWriter(s.discoveryEndpoint))
	}

	s.server = httptest.NewUnstartedServer(s.router)
	s.server.EnableHTTP2 = true
}

func (s *KubeMockServer) Close() error {
	s.server.Close()
	return nil
}

func (s *KubeMockServer) withWriter(handler httplib.HandlerFunc) httprouter.Handle {
	return httplib.MakeHandlerWithErrorWriter(handler, s.formatResponseError)
}

func (s *KubeMockServer) formatResponseError(rw http.ResponseWriter, respErr error) {
	status := &metav1.Status{
		Status: metav1.StatusFailure,
		// Don't trace.Unwrap the error, in case it was wrapped with a
		// user-friendly message. The underlying root error is likely too
		// low-level to be useful.
		Message: respErr.Error(),
		Code:    int32(trace.ErrorToCode(respErr)),
	}
	s.writeResponseError(rw, respErr, status)
}

func (s *KubeMockServer) writeResponseError(rw http.ResponseWriter, respErr error, status *metav1.Status) {
	data, err := runtime.Encode(kubeCodecs.LegacyCodec(), status)
	if err != nil {
		s.log.Warningf("Failed encoding error into kube Status object: %v", err)
		trace.WriteError(rw, respErr)
		return
	}
	rw.Header().Set(responsewriters.ContentTypeHeader, "application/json")
	// Always write InternalServerError, that's the only code that kubectl will
	// parse the Status object for. The Status object has the real status code
	// embedded.
	rw.WriteHeader(int(status.Code))
	if _, err := rw.Write(data); err != nil {
		s.log.Warningf("Failed writing kube error response body: %v", err)
	}
}

func (s *KubeMockServer) exec(w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	if wsstream.IsWebSocketRequest(req) {
		s.KubeExecRequests.Websocket.Add(1)
	} else {
		s.KubeExecRequests.SPDY.Add(1)
	}

	q := req.URL.Query()
	if s.execPodError != nil {
		s.writeResponseError(w, nil, s.execPodError)
		return nil, nil
	}
	request := remoteCommandRequest{
		namespace:          p.ByName("namespace"),
		name:               p.ByName("name"),
		containerName:      q.Get("container"),
		cmd:                q["command"],
		stdin:              utils.AsBool(q.Get("stdin")),
		stdout:             utils.AsBool(q.Get("stdout")),
		stderr:             utils.AsBool(q.Get("stderr")),
		tty:                utils.AsBool(q.Get("tty")),
		httpRequest:        req,
		httpResponseWriter: w,
		context:            req.Context(),
		pingPeriod:         defaults.HighResPollingPeriod,
		onResize:           func(remotecommand.TerminalSize) {},
	}

	proxy, err := createRemoteCommandProxy(request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxy.Close()

	if request.stdout {
		if _, err := proxy.stdoutStream.Write([]byte(request.containerName + "\n")); err != nil {
			s.log.WithError(err).Errorf("unable to send to stdout")
		}
	}

	if request.stderr {
		if _, err := proxy.stderrStream.Write([]byte(request.containerName + "\n")); err != nil {
			s.log.WithError(err).Errorf("unable to send to stderr")
		}
	}

	if request.stdin {
		buffer := make([]byte, 32*1024)
		for {
			buffer = buffer[:cap(buffer)]
			n, err := proxy.stdinStream.Read(buffer)
			if errors.Is(err, io.EOF) && n == 0 {
				break
			} else if err != nil && n == 0 {
				s.log.WithError(err).Errorf("unable to receive from stdin")
				break
			}

			buffer = buffer[:n]
			// Unfortunately, K8S Websocket protocol does not support half closed streams,
			// i.e. indicating that nothing else will be sent via stdin. If the server
			// reads the stdin stream until io.EOF is received, it will block on reading.
			// This issue is being tracked by https://github.com/kubernetes/kubernetes/issues/89899
			// In order to prevent this issue, and uniquely for the purpose of testing,
			// this server expects an exit keyword specified by CloseStreamMessage.
			// Once the exit is received, the server stops reading stdin.
			if bytes.Equal(buffer, []byte(CloseStreamMessage)) {
				break
			}

			if request.stdout {
				if _, err := proxy.stdoutStream.Write(buffer); err != nil {
					s.log.WithError(err).Errorf("unable to send to stdout")
				}
			}

			if request.stderr {
				if _, err := proxy.stderrStream.Write(buffer); err != nil {
					s.log.WithError(err).Errorf("unable to send to stdout")
				}
			}

		}

	}

	return nil, nil
}

// remoteCommandRequest is a request to execute a remote command
type remoteCommandRequest struct {
	namespace          string
	name               string
	containerName      string
	cmd                []string
	stdin              bool
	stdout             bool
	stderr             bool
	tty                bool
	httpRequest        *http.Request
	httpResponseWriter http.ResponseWriter
	onResize           resizeCallback
	context            context.Context
	pingPeriod         time.Duration
}

func createRemoteCommandProxy(req remoteCommandRequest) (*remoteCommandProxy, error) {
	var (
		proxy *remoteCommandProxy
		err   error
	)
	if wsstream.IsWebSocketRequest(req.httpRequest) {
		proxy, err = createWebSocketStreams(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		proxy, err = createSPDYStreams(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if proxy.resizeStream != nil {
		proxy.resizeQueue = newTermQueue(req.context, req.onResize)
		go proxy.resizeQueue.handleResizeEvents(proxy.resizeStream)
	}
	return proxy, nil
}

func channelOrIgnore(channel wsstream.ChannelType, real bool) wsstream.ChannelType {
	if real {
		return channel
	}
	return wsstream.IgnoreChannel
}

func createWebSocketStreams(req remoteCommandRequest) (*remoteCommandProxy, error) {
	channels := make([]wsstream.ChannelType, 5)
	channels[apiremotecommand.StreamStdIn] = channelOrIgnore(wsstream.ReadChannel, req.stdin)
	channels[apiremotecommand.StreamStdOut] = channelOrIgnore(wsstream.WriteChannel, req.stdout)
	channels[apiremotecommand.StreamStdErr] = channelOrIgnore(wsstream.WriteChannel, req.stderr)
	channels[apiremotecommand.StreamErr] = wsstream.WriteChannel
	channels[apiremotecommand.StreamResize] = wsstream.ReadChannel

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
	conn.SetIdleTimeout(IdleTimeout)
	_, streams, err := conn.Open(
		responsewriter.GetOriginal(req.httpResponseWriter),
		req.httpRequest,
	)
	if err != nil {
		return nil, trace.Wrap(err, "unable to upgrade websocket connection")
	}

	// Send an empty message to the lowest writable channel to notify the client the connection is established
	switch {
	case req.stdout:
		streams[apiremotecommand.StreamStdOut].Write([]byte{})
	case req.stderr:
		streams[apiremotecommand.StreamStdErr].Write([]byte{})
	default:
		streams[apiremotecommand.StreamErr].Write([]byte{})
	}

	proxy := &remoteCommandProxy{
		conn:         conn,
		stdinStream:  streams[apiremotecommand.StreamStdIn],
		stdoutStream: streams[apiremotecommand.StreamStdOut],
		stderrStream: streams[apiremotecommand.StreamStdErr],
		tty:          req.tty,
		resizeStream: streams[apiremotecommand.StreamResize],
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

	proxy.writeStatus = v4WriteStatusFunc(streams[apiremotecommand.StreamErr])

	return proxy, nil
}

func createSPDYStreams(req remoteCommandRequest) (*remoteCommandProxy, error) {
	protocol, err := httpstream.Handshake(req.httpRequest, req.httpResponseWriter, []string{StreamProtocolV4Name})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	streamCh := make(chan streamAndReply)

	upgrader := spdystream.NewResponseUpgraderWithPings(req.pingPeriod)
	conn := upgrader.UpgradeResponse(req.httpResponseWriter, req.httpRequest, func(stream httpstream.Stream, replySent <-chan struct{}) error {
		select {
		case streamCh <- streamAndReply{Stream: stream, replySent: replySent}:
			return nil
		case <-req.context.Done():
			return trace.Wrap(req.context.Err())
		}
	})
	// from this point on, we can no longer call methods on response
	if conn == nil {
		// The upgrader is responsible for notifying the client of any errors that
		// occurred during upgrading. All we can do is return here at this point
		// if we weren't successful in upgrading.
		return nil, trace.ConnectionProblem(trace.BadParameter("missing connection"), "missing connection")
	}

	conn.SetIdleTimeout(IdleTimeout)

	var handler protocolHandler
	switch protocol {
	case "":
		log.Warningf("Client did not request protocol negotiation.")
		fallthrough
	case StreamProtocolV4Name:
		log.Infof("Negotiated protocol %v.", protocol)
		handler = &v4ProtocolHandler{}
	default:
		return nil, trace.BadParameter("protocol %v is not supported. upgrade the client", protocol)
	}

	// count the streams client asked for, starting with 1
	expectedStreams := 1
	if req.stdin {
		expectedStreams++
	}
	if req.stdout {
		expectedStreams++
	}
	if req.stderr {
		expectedStreams++
	}
	if req.tty && handler.supportsTerminalResizing() {
		expectedStreams++
	}

	expired := time.NewTimer(DefaultStreamCreationTimeout)
	defer expired.Stop()

	proxy, err := handler.waitForStreams(req.context, streamCh, expectedStreams, expired.C)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxy.conn = conn
	proxy.tty = req.tty
	return proxy, nil
}

// remoteCommandProxy contains the connection and streams used when
// forwarding an attach or execute session into a container.
type remoteCommandProxy struct {
	conn         io.Closer
	stdinStream  io.ReadCloser
	stdoutStream io.WriteCloser
	stderrStream io.WriteCloser
	writeStatus  func(status *apierrors.StatusError) error
	resizeStream io.ReadCloser
	tty          bool
	resizeQueue  *termQueue
}

func (s *remoteCommandProxy) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// streamAndReply holds both a Stream and a channel that is closed when the stream's reply frame is
// enqueued. Consumers can wait for replySent to be closed prior to proceeding, to ensure that the
// replyFrame is enqueued before the connection's goaway frame is sent (e.g. if a stream was
// received and right after, the connection gets closed).
type streamAndReply struct {
	httpstream.Stream
	replySent <-chan struct{}
}

func newTermQueue(parentContext context.Context, onResize resizeCallback) *termQueue {
	ctx, cancel := context.WithCancel(parentContext)
	return &termQueue{
		ch:       make(chan remotecommand.TerminalSize),
		cancel:   cancel,
		done:     ctx,
		onResize: onResize,
	}
}

type resizeCallback func(remotecommand.TerminalSize)

type termQueue struct {
	ch       chan remotecommand.TerminalSize
	cancel   context.CancelFunc
	done     context.Context
	onResize resizeCallback
}

func (t *termQueue) Next() *remotecommand.TerminalSize {
	select {
	case size := <-t.ch:
		t.onResize(size)
		return &size
	case <-t.done.Done():
		return nil
	}
}

func (t *termQueue) Done() {
	t.cancel()
}

func (t *termQueue) handleResizeEvents(stream io.Reader) {
	decoder := json.NewDecoder(stream)
	for {
		size := remotecommand.TerminalSize{}
		if err := decoder.Decode(&size); err != nil {
			if !errors.Is(err, io.EOF) {
				log.Warningf("Failed to decode resize event: %v", err)
			}
			t.cancel()
			return
		}
		select {
		case t.ch <- size:
		case <-t.done.Done():
			return
		}
	}
}

type protocolHandler interface {
	// waitForStreams waits for the expected streams or a timeout, returning a
	// remoteCommandContext if all the streams were received, or an error if not.
	waitForStreams(ctx context.Context, streams <-chan streamAndReply, expectedStreams int, expired <-chan time.Time) (*remoteCommandProxy, error)
	// supportsTerminalResizing returns true if the protocol handler supports terminal resizing
	supportsTerminalResizing() bool
}

// v4ProtocolHandler implements the V4 protocol version for streaming command execution. It only differs
// in from v3 in the error stream format using an json-marshaled metav1.Status which carries
// the process' exit code.
type v4ProtocolHandler struct{}

func (*v4ProtocolHandler) waitForStreams(connContext context.Context, streams <-chan streamAndReply, expectedStreams int, expired <-chan time.Time) (*remoteCommandProxy, error) {
	remoteProxy := &remoteCommandProxy{}
	receivedStreams := 0
	replyChan := make(chan struct{})

	stopCtx, cancel := context.WithCancel(connContext)
	defer cancel()
WaitForStreams:
	for {
		select {
		case stream := <-streams:
			streamType := stream.Headers().Get(StreamType)
			switch streamType {
			case StreamTypeError:
				remoteProxy.writeStatus = v4WriteStatusFunc(stream)
				go waitStreamReply(stopCtx, stream.replySent, replyChan)
			case StreamTypeStdin:
				remoteProxy.stdinStream = stream
				go waitStreamReply(stopCtx, stream.replySent, replyChan)
			case StreamTypeStdout:
				remoteProxy.stdoutStream = stream
				go waitStreamReply(stopCtx, stream.replySent, replyChan)
			case StreamTypeStderr:
				remoteProxy.stderrStream = stream
				go waitStreamReply(stopCtx, stream.replySent, replyChan)
			case StreamTypeResize:
				remoteProxy.resizeStream = stream
				go waitStreamReply(stopCtx, stream.replySent, replyChan)
			default:
				log.Warningf("Ignoring unexpected stream type: %q", streamType)
			}
		case <-replyChan:
			receivedStreams++
			if receivedStreams == expectedStreams {
				break WaitForStreams
			}
		case <-expired:
			return nil, trace.BadParameter("timed out waiting for client to create streams")
		case <-connContext.Done():
			return nil, trace.BadParameter("connection has dropped, exiting")
		}
	}

	return remoteProxy, nil
}

// supportsTerminalResizing returns true because v4ProtocolHandler supports it
func (*v4ProtocolHandler) supportsTerminalResizing() bool { return true }

// waitStreamReply waits until either replySent or stop is closed. If replySent is closed, it sends
// an empty struct to the notify channel.
func waitStreamReply(ctx context.Context, replySent <-chan struct{}, notify chan<- struct{}) {
	select {
	case <-replySent:
		notify <- struct{}{}
	case <-ctx.Done():
	}
}

// v4WriteStatusFunc returns a WriteStatusFunc that marshals a given api Status
// as json in the error channel.
func v4WriteStatusFunc(stream io.Writer) func(status *apierrors.StatusError) error {
	return func(status *apierrors.StatusError) error {
		bs, err := json.Marshal(status.Status())
		if err != nil {
			return err
		}
		_, err = stream.Write(bs)
		return err
	}
}

func (s *KubeMockServer) selfSubjectAccessReviews(w http.ResponseWriter, req *http.Request, p httprouter.Params) (resp any, err error) {
	s1 := &v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Verb: "impersonate",
			},
		},
		Status: v1.SubjectAccessReviewStatus{
			Allowed: true,
			Denied:  false,
			Reason:  "RBAC: allowed",
		},
	}

	return s1, nil
}

// portforward supports SPDY protocols only. Teleport always uses SPDY when
// portforwarding to upstreams even if the original request is WebSocket.
func (s *KubeMockServer) portforward(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	_, err := httpstream.Handshake(req, w, []string{portForwardProtocolV1Name})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	streamChan := make(chan httpstream.Stream)

	upgrader := spdystream.NewResponseUpgraderWithPings(defaults.HighResPollingPeriod)
	conn := upgrader.UpgradeResponse(w, req, httpStreamReceived(req.Context(), streamChan))
	if conn == nil {
		err = trace.ConnectionProblem(nil, "unable to upgrade SPDY connection")
		return nil, err
	}
	defer conn.Close()
	var (
		data      httpstream.Stream
		errStream httpstream.Stream
	)

	for {
		select {
		case <-conn.CloseChan():
			return nil, nil
		case stream := <-streamChan:
			switch stream.Headers().Get(StreamType) {
			case StreamTypeError:
				errStream = stream
			case StreamTypeData:
				data = stream
			}
		}
		if errStream != nil && data != nil {
			break
		}
	}

	buf := make([]byte, 1024)
	n, err := data.Read(buf)
	if err != nil {
		errStream.Write([]byte(err.Error()))
		return nil, nil
	}
	fmt.Fprint(data, PortForwardPayload, p.ByName("name"), string(buf[:n]))
	return nil, nil
}

// httpStreamReceived is the httpstream.NewStreamHandler for port
// forward streams. It checks each stream's port and stream type headers,
// rejecting any streams that with missing or invalid values. Each valid
// stream is sent to the streams channel.
func httpStreamReceived(ctx context.Context, streams chan httpstream.Stream) func(httpstream.Stream, <-chan struct{}) error {
	return func(stream httpstream.Stream, _ <-chan struct{}) error {
		// make sure it has a valid port header
		portString := stream.Headers().Get(portHeader)
		if len(portString) == 0 {
			return trace.BadParameter("%q header is required", portHeader)
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

func (s *KubeMockServer) versionEndpoint(_ http.ResponseWriter, _ *http.Request, _ httprouter.Params) (resp any, err error) {
	if s.version == nil {
		return nil, trace.BadParameter("version not set")
	}
	return s.version, nil
}
