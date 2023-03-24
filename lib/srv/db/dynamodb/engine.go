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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/dax"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodbstreams"
	"github.com/gravitational/trace"

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
// https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.Errors.html#Programming.Errors.Components
type jsonErr struct {
	Code    string `json:"__type"`
	Message string `json:"message"`
}

// SendError sends an error to DynamoDB client.
func (e *Engine) SendError(err error) {
	if e.clientConn == nil || err == nil || utils.IsOKNetworkError(err) {
		return
	}
	e.Log.WithError(err).Error("DynamoDB connection error")

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

// process reads request from connected dynamodb client, processes the requests/responses and sends data back
// to the client.
func (e *Engine) process(ctx context.Context, req *http.Request) (err error) {
	if req.Body != nil {
		// make sure we close the incoming request's body. ignore any close error.
		defer req.Body.Close()
	}

	re, err := e.resolveEndpoint(req)
	if err != nil {
		// special error case where we couldn't resolve the endpoint, just emit using the configured URI.
		e.emitAuditEvent(req, e.sessionCtx.Database.GetURI(), 0, err)
		return trace.Wrap(err)
	}

	// emit an audit event regardless of failure, but using the resolved endpoint.
	var responseStatusCode uint32
	defer func() {
		e.emitAuditEvent(req, re.URL, responseStatusCode, err)
	}()

	// try to read, close, and replace the incoming request body.
	body, err := utils.GetAndReplaceRequestBody(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundTripper, err := e.getRoundTripper(ctx, re.URL)
	if err != nil {
		return trace.Wrap(err)
	}
	// rewrite the request URL and headers before signing it.
	outReq, err := rewriteRequest(ctx, req, re, body)
	if err != nil {
		return trace.Wrap(err)
	}

	roleArn, err := libaws.BuildRoleARN(e.sessionCtx.DatabaseUser,
		re.SigningRegion, e.sessionCtx.Database.GetAWS().AccountID)
	if err != nil {
		return trace.Wrap(err)
	}
	signedReq, err := e.signingSvc.SignRequest(e.Context, outReq,
		&libaws.SigningCtx{
			SigningName:   re.SigningName,
			SigningRegion: re.SigningRegion,
			Expiry:        e.sessionCtx.Identity.Expires,
			SessionName:   e.sessionCtx.Identity.Username,
			AWSRoleArn:    roleArn,
			AWSExternalID: e.sessionCtx.Database.GetAWS().ExternalID,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	// Send the request.
	resp, err := roundTripper.RoundTrip(signedReq)
	if err != nil {
		// convert the error from round tripping to try to get a trace error.
		err = common.ConvertConnectError(err, e.sessionCtx)
		return trace.Wrap(err)
	}
	defer resp.Body.Close()
	responseStatusCode = uint32(resp.StatusCode)

	return trace.Wrap(e.sendResponse(resp))
}

// sendResponse sends the response back to the DynamoDB client.
func (e *Engine) sendResponse(resp *http.Response) error {
	return trace.Wrap(resp.Write(e.clientConn))
}

// emitAuditEvent writes the request and response status code to the audit stream.
func (e *Engine) emitAuditEvent(req *http.Request, uri string, statusCode uint32, err error) {
	var eventCode string
	if err == nil && statusCode != 0 {
		eventCode = events.DynamoDBRequestCode
	} else {
		eventCode = events.DynamoDBRequestFailureCode
	}
	// Try to read the body and JSON unmarshal it.
	// If this fails, we still want to emit the rest of the event info; the request event Body is nullable,
	// so it's ok if body is nil here.
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
			Code: eventCode,
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
		StatusCode: statusCode,
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
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.DatabaseRoleMatchers(
		sessionCtx.Database,
		sessionCtx.DatabaseUser,
		sessionCtx.DatabaseName,
	)
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	return trace.Wrap(err)
}

// getRoundTripper makes an HTTP round tripper with TLS config based on the given URL.
func (e *Engine) getRoundTripper(ctx context.Context, URL string) (http.RoundTripper, error) {
	if rt, ok := e.RoundTrippers[URL]; ok {
		return rt, nil
	}
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// We need to set the ServerName here because the AWS endpoint service prefix is not known in advance,
	// and the TLS config we got does not set it.
	host, err := getURLHostname(URL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.ServerName = host

	out, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out.TLSClientConfig = tlsConfig
	e.RoundTrippers[URL] = out
	return out, nil
}

// resolveEndpoint returns a resolved endpoint for either the configured URI or the AWS target service and region.
func (e *Engine) resolveEndpoint(req *http.Request) (*endpoints.ResolvedEndpoint, error) {
	endpointID, err := extractEndpointID(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opts := func(opts *endpoints.Options) {
		opts.ResolveUnknownService = true
	}
	re, err := endpoints.DefaultResolver().EndpointFor(endpointID, e.sessionCtx.Database.GetAWS().Region, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uri := e.sessionCtx.Database.GetURI()
	if uri != "" && uri != apiaws.DynamoDBURIForRegion(e.sessionCtx.Database.GetAWS().Region) {
		// override the resolved endpoint URL with the user-configured URI.
		re.URL = uri
	}
	if !strings.Contains(re.URL, "://") {
		re.URL = "https://" + re.URL
	}
	return &re, nil
}

// rewriteRequest clones a request, modifies the clone to rewrite its URL, and returns the modified request clone.
func rewriteRequest(ctx context.Context, r *http.Request, re *endpoints.ResolvedEndpoint, body []byte) (*http.Request, error) {
	resolvedURL, err := url.Parse(re.URL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqCopy := r.Clone(ctx)
	// set url and host header to match the resolved endpoint.
	reqCopy.URL = resolvedURL
	reqCopy.Host = resolvedURL.Host
	if body == nil {
		// no body is fine, skip copying it.
		return reqCopy, nil
	}

	// copy request body
	reqCopy.Body = io.NopCloser(bytes.NewReader(body))
	return reqCopy, nil
}

// extractEndpointID extracts the AWS endpoint ID from the request header X-Amz-Target.
func extractEndpointID(req *http.Request) (string, error) {
	target := req.Header.Get(libaws.AmzTargetHeader)
	if target == "" {
		return "", trace.BadParameter("missing %q header in http request", libaws.AmzTargetHeader)
	}
	endpointID, err := endpointIDForTarget(target)
	return endpointID, trace.Wrap(err)
}

// endpointIDForTarget converts a target operation into the appropriate the AWS endpoint ID.
// Target looks like one of DynamoDB_$version.$operation, DynamoDBStreams_$version.$operation, AmazonDAX$version.$operation,
// for example: DynamoDBStreams_20120810.ListStreams
// See X-Amz-Target: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.LowLevelAPI.html
func endpointIDForTarget(target string) (string, error) {
	t := strings.ToLower(target)
	switch {
	case strings.HasPrefix(t, "dynamodbstreams"):
		return dynamodbstreams.EndpointsID, nil
	case strings.HasPrefix(t, "dynamodb"):
		return dynamodb.EndpointsID, nil
	case strings.HasPrefix(t, "amazondax"):
		return dax.EndpointsID, nil
	default:
		return "", trace.BadParameter("DynamoDB API target %q is not recognized", target)
	}
}

// getURLHostname parses a URL to extract its hostname.
func getURLHostname(uri string) (string, error) {
	if !strings.Contains(uri, "://") {
		uri = "schema://" + uri
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return parsed.Hostname(), nil
}
