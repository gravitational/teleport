package remotewebsocket

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	gwebsocket "github.com/gorilla/websocket"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/httpstream"
	machineryremotecommand "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

// CheckIfWebSocketsAreSupported checks if the server supports websockets by connecting and checking the negotiated protocol.
func CheckIfWebSocketsAreSupported(
	config *rest.Config, method string, u *url.URL,
) error {

	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return err
	}
	rt, wsRt, err := RoundTripperFor(config)
	if err != nil {
		return err
	}
	conn, err := Negotiate(rt, wsRt, req, machineryremotecommand.StreamProtocolV4Name, machineryremotecommand.StreamProtocolV1Name)
	if err != nil {
		return err
	}
	defer conn.Close()
	streamingProto := conn.Subprotocol()

	switch streamingProto {
	case machineryremotecommand.StreamProtocolV4Name, machineryremotecommand.StreamProtocolV1Name:
		return nil
	default:
		return fmt.Errorf("unsupported streaming protocol: %q", streamingProto)
	}
}

func RoundTripperFor(config *rest.Config) (http.RoundTripper, *RoundTripper, error) {
	transportCfg, err := config.TransportConfig()
	if err != nil {
		return nil, nil, err
	}
	tlsConfig, err := transport.TLSConfigFor(transportCfg)
	if err != nil {
		return nil, nil, err
	}

	upgradeRoundTripper := &RoundTripper{
		TLSConfig: tlsConfig,
	}
	wrapper, err := transport.HTTPWrappersForConfig(transportCfg, upgradeRoundTripper)
	if err != nil {
		return nil, nil, err
	}
	return wrapper, upgradeRoundTripper, nil
}

func Negotiate(rt http.RoundTripper, wsRt *RoundTripper, req *http.Request, protocols ...string) (*gwebsocket.Conn, error) {
	req.Header[httpstream.HeaderProtocolVersion] = protocols
	resp, err := rt.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	err = resp.Body.Close()
	if err != nil {
		wsRt.Conn.Close()
		return nil, fmt.Errorf("error closing response body: %v", err)
	}
	return wsRt.Conn, nil
}

// RoundTripper knows how to establish a connection to a remote WebSocket endpoint and make it available for use.
// RoundTripper must not be reused.
type RoundTripper struct {
	// TLSConfig holds the TLS configuration settings to use when connecting
	// to the remote server.
	TLSConfig *tls.Config

	// Proxier specifies a function to return a proxy for a given
	// Request. If the function returns a non-nil error, the
	// request is aborted with the provided error.
	// If Proxy is nil or returns a nil *URL, no proxy is used.
	Proxier func(req *http.Request) (*url.URL, error)

	// Conn holds the WebSocket connection after a round trip.
	Conn *gwebsocket.Conn
}

// TLSClientConfig implements pkg/util/net.TLSClientConfigHolder.
func (rt *RoundTripper) TLSClientConfig() *tls.Config {
	return rt.TLSConfig
}

// RoundTrip connects to the remote websocket using the headers in the request and the TLS
// configuration from the config
func (rt *RoundTripper) RoundTrip(request *http.Request) (retResp *http.Response, retErr error) {
	defer func() {
		if request.Body != nil {
			err := request.Body.Close()
			if retErr == nil {
				retErr = err
			}
		}
	}()

	// set the protocol version directly on the dialer from the header
	protocolVersions := request.Header[httpstream.HeaderProtocolVersion]
	delete(request.Header, httpstream.HeaderProtocolVersion)

	dialer := gwebsocket.Dialer{
		Proxy:           rt.Proxier,
		TLSClientConfig: rt.TLSConfig,
		Subprotocols:    protocolVersions,
	}
	switch request.URL.Scheme {
	case "https":
		request.URL.Scheme = "wss"
	case "http":
		request.URL.Scheme = "ws"
	}
	wsConn, resp, err := dialer.DialContext(request.Context(), request.URL.String(), request.Header)
	if err != nil {
		if err != gwebsocket.ErrBadHandshake {
			return nil, err
		}
		// resp.Body contains (a part of) the response when err == gwebsocket.ErrBadHandshake
		responseErrorBytes, bodyErr := ioutil.ReadAll(resp.Body)
		if bodyErr != nil {
			return nil, err
		}
		// TODO: I don't belong here, I should be abstracted from this class
		if obj, _, err := statusCodecs.UniversalDecoder().Decode(responseErrorBytes, nil, &metav1.Status{}); err == nil {
			if status, ok := obj.(*metav1.Status); ok {
				return nil, &apierrors.StatusError{ErrStatus: *status}
			}
		}
		return nil, fmt.Errorf("unable to upgrade connection: %s", bytes.TrimSpace(responseErrorBytes))
	}

	rt.Conn = wsConn

	return resp, nil
}

// statusScheme is private scheme for the decoding here until someone fixes the TODO in NewConnection
var statusScheme = runtime.NewScheme()

// ParameterCodec knows about query parameters used with the meta v1 API spec.
var statusCodecs = serializer.NewCodecFactory(statusScheme)
