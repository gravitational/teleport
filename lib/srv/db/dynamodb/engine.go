/*

 Copyright 2022 Gravitational, Inc.

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

package dynamodb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/service/dax"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiaws "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
	"github.com/gravitational/teleport/lib/utils"
	libaws "github.com/gravitational/teleport/lib/utils/aws"
)

// NewEngine create new DynamoDB engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig:  ec,
		RoundTrippers: make(map[string]http.RoundTripper),
	}
}

// Engine handles connections from DynamoDB clients coming from Teleport
// proxy over reverse tunnel.
type Engine struct {
	// signingSvc will be used by the engine to provide the AWS sigv4 authorization header
	// required by AWS for request validation: https://docs.aws.amazon.com/general/latest/gr/signing-elements.html
	signingSvc *libaws.SigningService
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// RoundTrippers is a cache of RoundTrippers, mapped by service endpoint.
	// It is not guarded by a mutex, since requests are processed serially.
	RoundTrippers map[string]http.RoundTripper
	// GetSigningCredsFn allows to set the function responsible for obtaining STS credentials.
	// Used in tests to set static AWS credentials and skip API call.
	GetSigningCredsFn libaws.GetSigningCredentialsFunc
}

var _ common.Engine = (*Engine)(nil)

func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
	awsSession, err := e.CloudClients.GetAWSSession(sessionCtx.Database.GetAWS().Region)
	if err != nil {
		return trace.Wrap(err)
	}
	svc, err := libaws.NewSigningService(libaws.SigningServiceConfig{
		Clock:                 e.Clock,
		Session:               awsSession,
		GetSigningCredentials: e.GetSigningCredsFn,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	e.signingSvc = svc
	return nil
}

// jsonErr is used to marshal a JSON error response as the AWS CLI expects for errors.
type jsonErr struct {
	Code    string `json:"__type"`
	Message string `json:"message"`
}

// SendError sends an error to DynamoDB client.
func (e *Engine) SendError(err error) {
	if e.clientConn == nil || err == nil || utils.IsOKNetworkError(err) {
		return
	}
	e.Log.Debugf("DynamoDB connection error: %v", err)

	// try to convert to a trace err if we can.
	code := trace.ErrorToCode(err)
	body, err := json.Marshal(jsonErr{
		Code:    strconv.Itoa(code),
		Message: err.Error(),
	})
	if err != nil {
		e.Log.WithError(err).Error("failed to marshal error response")
		return
	}
	response := &http.Response{
		Status:     http.StatusText(code),
		StatusCode: code,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		// ContentLength is the authoritative value in the response,
		// no need to also add the "Content-Length" header (source: go doc http.Response.Header).
		ContentLength: int64(len(body)),
		Header: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: io.NopCloser(bytes.NewBuffer(body)),
	}

	if err := response.Write(e.clientConn); err != nil {
		e.Log.WithError(err).Error("failed to send error response to DynamoDB client")
		return
	}
}

// HandleConnection authorizes the incoming client connection, connects to the
// target DynamoDB server and starts proxying requests between client/server.
func (e *Engine) HandleConnection(ctx context.Context, _ *common.Session) error {
	err := e.checkAccess(ctx, e.sessionCtx)
	e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
	if err != nil {
		return trace.Wrap(err)
	}
	defer e.Audit.OnSessionEnd(e.Context, e.sessionCtx)

	clientConnReader := bufio.NewReader(e.clientConn)
	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := e.process(ctx, req); err != nil {
			return trace.Wrap(err)
		}
	}
}

// process reads request from connected dynamodb client, processes the requests/responses and send data back
// to the client.
func (e *Engine) process(ctx context.Context, req *http.Request) error {
	defer req.Body.Close()
	body, err := io.ReadAll(io.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
	if err != nil {
		return trace.Wrap(err)
	}

	service, err := e.getService(req)
	if err != nil {
		return trace.Wrap(err)
	}
	signingName, err := serviceToSigningName(service)
	if err != nil {
		return trace.Wrap(err)
	}
	uri, err := e.getTargetURI(service)
	if err != nil {
		return trace.Wrap(err)
	}
	roundTripper, err := e.getRoundTripper(ctx, uri)
	if err != nil {
		return trace.Wrap(err)
	}
	// rewrite the request URL and headers before signing it.
	reqCopy, err := rewriteRequest(ctx, req, uri, bytes.NewReader(body))
	if err != nil {
		return trace.Wrap(err)
	}

	region := e.sessionCtx.Database.GetAWS().Region
	roleArn := libaws.BuildRoleARN(e.sessionCtx.DatabaseUser, region, e.sessionCtx.Database.GetAWS().AccountID)
	signedReq, err := e.signingSvc.SignRequest(reqCopy,
		&libaws.SigningCtx{
			SigningName:   signingName,
			SigningRegion: region,
			Expiry:        e.sessionCtx.Identity.Expires,
			SessionName:   e.sessionCtx.Identity.Username,
			AWSRoleArn:    roleArn,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	// Send the request to DynamoDB API.
	resp, err := roundTripper.RoundTrip(signedReq)
	if err != nil {
		// convert the error from round tripping to try to get a trace error.
		err = common.ConvertConnectError(err, e.sessionCtx)
		e.Log.WithError(err).Error("Request failed.")
		return trace.Wrap(err)
	}
	defer resp.Body.Close()

	e.emitAuditEvent(reqCopy, uri, resp.StatusCode)
	return trace.Wrap(e.sendResponse(resp))
}

// sendResponse sends the response back to the DynamoDB client.
func (e *Engine) sendResponse(resp *http.Response) error {
	return trace.Wrap(resp.Write(e.clientConn))
}

// emitAuditEvent writes the request and response status code to the audit stream.
func (e *Engine) emitAuditEvent(req *http.Request, uri string, statusCode int) {
	// Try to read the body and JSON unmarshal it.
	// If this fails, we still want to emit the rest of the event info; the request event Body is nullable, so it's ok if body is left nil here.
	body, err := libaws.UnmarshalRequestBody(req)
	if err != nil {
		e.Log.WithError(err).Warn("Failed to read request body as JSON, omitting the body from the audit event.")
	}
	// get the API target from the request header, according to the API request format documentation:
	// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.LowLevelAPI.html#Programming.LowLevelAPI.RequestFormat
	target := req.Header.Get(libaws.AmzTargetHeader)
	event := &apievents.DynamoDBRequest{
		Metadata: apievents.Metadata{
			Type: events.DatabaseSessionDynamoDBRequestEvent,
			Code: events.DynamoDBRequestCode,
		},
		UserMetadata:    e.sessionCtx.Identity.GetUserMetadata(),
		SessionMetadata: common.MakeSessionMetadata(e.sessionCtx),
		DatabaseMetadata: apievents.DatabaseMetadata{
			DatabaseService:  e.sessionCtx.Database.GetName(),
			DatabaseProtocol: e.sessionCtx.Database.GetProtocol(),
			DatabaseURI:      uri,
			DatabaseName:     e.sessionCtx.DatabaseName,
			DatabaseUser:     e.sessionCtx.DatabaseUser,
		},
		StatusCode: uint32(statusCode),
		Path:       req.URL.Path,
		RawQuery:   req.URL.RawQuery,
		Method:     req.Method,
		Target:     target,
		Body:       body,
	}
	e.Audit.EmitEvent(e.Context, event)
}

// checkAccess does authorization check for DynamoDB connection about
// to be established.
func (e *Engine) checkAccess(ctx context.Context, sessionCtx *common.Session) error {
	ap, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	mfaParams := sessionCtx.MFAParams(ap.GetRequireMFAType())
	dbRoleMatchers := role.DatabaseRoleMatchers(
		sessionCtx.Database.GetProtocol(),
		sessionCtx.DatabaseUser,
		sessionCtx.DatabaseName,
	)
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		mfaParams,
		dbRoleMatchers...,
	)
	return trace.Wrap(err)
}

// getRoundTripper makes an HTTP client with TLS config based on the given URI host.
func (e *Engine) getRoundTripper(ctx context.Context, uri string) (http.RoundTripper, error) {
	if rt, ok := e.RoundTrippers[uri]; ok {
		return rt, nil
	}
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostname, err := getURIHostname(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// We need to set the ServerName here because the AWS endpoint service prefix is not known in advance,
	// and the TLS config we got does not set it.
	tlsConfig.ServerName = hostname

	out, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out.TLSClientConfig = tlsConfig
	e.RoundTrippers[uri] = out
	return out, nil
}

// getService extracts the service ID from the request header X-Amz-Target.
func (e *Engine) getService(req *http.Request) (string, error) {
	// try to get the service from x-amz-target header.
	target := req.Header.Get(libaws.AmzTargetHeader)
	if target == "" {
		return "", trace.BadParameter("missing %q header in http request", libaws.AmzTargetHeader)
	}
	service, err := parseDynamoDBServiceFromTarget(target)
	return service, trace.Wrap(err)
}

// getTargetURI returns the target URI constructed from a given service.
func (e *Engine) getTargetURI(service string) (string, error) {
	uri := e.sessionCtx.Database.GetURI()
	if !apiaws.IsAWSEndpoint(uri) {
		// non-aws endpoints are used without modification. This is an unusual configuration but supports unforeseen use-cases.
		return uri, nil
	}
	// when the database is created we ensure any AWS endpoint is configured to be a suffix missing only the service prefix.
	prefix, err := endpointPrefixForService(service)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return prefix + uri, nil
}

// rewriteRequest rewrites the request to modify headers and the URL.
func rewriteRequest(ctx context.Context, r *http.Request, uri string, body io.Reader) (*http.Request, error) {
	reqCopy, err := http.NewRequestWithContext(ctx, r.Method, r.URL.String(), body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqCopy.Header = r.Header.Clone()

	// force https and set url and host header to match the database uri.
	reqCopy.URL.Scheme = "https"
	reqCopy.Host = uri
	reqCopy.URL.Host = uri

	for key := range reqCopy.Header {
		// Remove Content-Length header for SigV4 signing.
		if http.CanonicalHeaderKey(key) == "Content-Length" {
			reqCopy.Header.Del(key)
		}
	}
	return reqCopy, nil
}

// getURIHostname parses a URI to extract its hostname.
func getURIHostname(uri string) (string, error) {
	if !strings.Contains(uri, "://") {
		uri = "schema://" + uri
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return parsed.Hostname(), nil
}

// parseDynamoDBServiceFromTarget parses the DynamoDB service ID given a target operation.
// Target looks like one of DynamoDB_$version.$operation, DynamoDBStreams_$version.$operation, AmazonDAX$version.$operation,
// for example: DynamoDBStreams_20120810.ListStreams
// See X-Amz-Target: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.LowLevelAPI.html
func parseDynamoDBServiceFromTarget(target string) (string, error) {
	t := strings.ToLower(target)
	switch {
	case strings.HasPrefix(t, "dynamodbstreams"):
		return dynamodbstreams.ServiceID, nil
	case strings.HasPrefix(t, "dynamodb"):
		return dynamodb.ServiceID, nil
	case strings.HasPrefix(t, "amazondax"):
		return dax.ServiceID, nil
	default:
		return "", trace.BadParameter("DynamoDB API target %q is not recognized", target)
	}
}

// endpointPrefixForService returns the prefix string used for a given AWS DynamoDB service.
// The endpoint prefix looks like one of "dynamodb", "dax", "streams.dynamodb".
func endpointPrefixForService(service string) (string, error) {
	switch service {
	case dynamodb.ServiceID:
		return dynamodb.ServiceName, nil
	case dynamodbstreams.ServiceID:
		return dynamodbstreams.ServiceName, nil
	case dax.ServiceID:
		return dax.ServiceName, nil
	default:
		return "", trace.BadParameter("unrecognized DynamoDB service %q", service)
	}
}

// serviceToSigningName converts a DynamoDB service ID to the appropriate signing name.
func serviceToSigningName(service string) (string, error) {
	switch service {
	case dynamodb.ServiceID, dynamodbstreams.ServiceID:
		return "dynamodb", nil
	case dax.ServiceID:
		return "dax", nil
	default:
		return "", trace.BadParameter("service %q is not recognized", service)
	}
}
