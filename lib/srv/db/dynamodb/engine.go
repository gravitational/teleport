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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dax"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiaws "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
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
		UseFIPS:       modules.GetModules().IsBoringBinary(),
	}
}

// Engine handles connections from DynamoDB clients coming from Teleport
// proxy over reverse tunnel.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// RoundTrippers is a cache of RoundTrippers, mapped by service endpoint.
	// It is not guarded by a mutex, since requests are processed serially.
	RoundTrippers map[string]http.RoundTripper
	// CredentialsGetter is used to obtain STS credentials.
	CredentialsGetter libaws.CredentialsGetter
	// UseFIPS will ensure FIPS endpoint resolution.
	UseFIPS bool
}

var _ common.Engine = (*Engine)(nil)

func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	e.sessionCtx = sessionCtx
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
	e.Log.ErrorContext(e.Context, "DynamoDB connection error", "error", err)

	// try to convert to a trace err if we can.
	code := trace.ErrorToCode(err)
	body, err := json.Marshal(jsonErr{
		Code:    strconv.Itoa(code),
		Message: err.Error(),
	})
	if err != nil {
		e.Log.ErrorContext(e.Context, "failed to marshal error response", "error", err)
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
		e.Log.ErrorContext(e.Context, "failed to send error response to DynamoDB client", "error", err)
		return
	}
}

// HandleConnection authorizes the incoming client connection, connects to the
// target DynamoDB server and starts proxying requests between client/server.
func (e *Engine) HandleConnection(ctx context.Context, _ *common.Session) error {
	observe := common.GetConnectionSetupTimeObserver(e.sessionCtx.Database)
	err := e.checkAccess(ctx, e.sessionCtx)
	e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
	if err != nil {
		return trace.Wrap(err)
	}
	defer e.Audit.OnSessionEnd(e.Context, e.sessionCtx)

	meta := e.sessionCtx.Database.GetAWS()
	awsSession, err := e.CloudClients.GetAWSSession(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	signer, err := libaws.NewSigningService(libaws.SigningServiceConfig{
		Clock:             e.Clock,
		SessionProvider:   libaws.StaticAWSSessionProvider(awsSession),
		CredentialsGetter: e.CredentialsGetter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clientConnReader := bufio.NewReader(e.clientConn)

	observe()

	msgFromClient := common.GetMessagesFromClientMetric(e.sessionCtx.Database)
	msgFromServer := common.GetMessagesFromServerMetric(e.sessionCtx.Database)

	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := e.process(ctx, req, signer, msgFromClient, msgFromServer); err != nil {
			return trace.Wrap(err)
		}
	}
}

// process reads request from connected dynamodb client, processes the requests/responses and sends data back
// to the client.
func (e *Engine) process(ctx context.Context, req *http.Request, signer *libaws.SigningService, msgFromClient prometheus.Counter, msgFromServer prometheus.Counter) (err error) {
	msgFromClient.Inc()

	if req.Body != nil {
		// make sure we close the incoming request's body. ignore any close error.
		defer req.Body.Close()
		req.Body = io.NopCloser(utils.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
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
		e.emitAuditEvent(req, re.URL.String(), responseStatusCode, err)
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

	meta := e.sessionCtx.Database.GetAWS()
	roleArn, err := libaws.BuildRoleARN(e.sessionCtx.DatabaseUser,
		re.SigningRegion, meta.AccountID)
	if err != nil {
		return trace.Wrap(err)
	}
	signingCtx := &libaws.SigningCtx{
		SigningName:   re.SigningName,
		SigningRegion: re.SigningRegion,
		Expiry:        e.sessionCtx.Identity.Expires,
		SessionName:   e.sessionCtx.Identity.Username,
		AWSRoleArn:    roleArn,
		SessionTags:   e.sessionCtx.Database.GetAWS().SessionTags,
	}
	if meta.AssumeRoleARN == "" {
		signingCtx.AWSExternalID = meta.ExternalID
	}
	signedReq, err := signer.SignRequest(e.Context, outReq, signingCtx)
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

	msgFromServer.Inc()

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
		e.Log.WarnContext(e.Context, "Failed to read request body as JSON, omitting the body from the audit event.", "error", err)
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
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:     sessionCtx.Database,
		DatabaseUser: sessionCtx.DatabaseUser,
		DatabaseName: sessionCtx.DatabaseName,
	})
	err = sessionCtx.Checker.CheckAccess(
		sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	return trace.Wrap(err)
}

// getRoundTripper makes an HTTP round tripper with TLS config based on the given URL.
func (e *Engine) getRoundTripper(ctx context.Context, u *url.URL) (http.RoundTripper, error) {
	if rt, ok := e.RoundTrippers[u.String()]; ok {
		return rt, nil
	}
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx.GetExpiry(), e.sessionCtx.Database, e.sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// We need to set the ServerName here because the AWS endpoint service prefix is not known in advance,
	// and the TLS config we got does not set it.
	tlsConfig.ServerName = u.Hostname()

	out, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out.TLSClientConfig = tlsConfig
	e.RoundTrippers[u.String()] = out
	return out, nil
}

type endpoint struct {
	URL           *url.URL
	SigningName   string
	SigningRegion string
}

// resolveEndpoint returns a resolved endpoint for either the configured URI or
// the AWS target service and region.
// For a target operation, the appropriate AWS service resolver is used.
// Targets look like one of DynamoDB_$version.$operation,
// DynamoDBStreams_$version.$operation, or AmazonDAX$version.$operation.
// For example: DynamoDBStreams_20120810.ListStreams
func (e *Engine) resolveEndpoint(req *http.Request) (*endpoint, error) {
	target, err := getTargetHeader(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsMeta := e.sessionCtx.Database.GetAWS()

	var re *endpoint
	switch target := strings.ToLower(target); {
	case strings.HasPrefix(target, "dynamodbstreams"):
		re, err = resolveDynamoDBStreamsEndpoint(req.Context(), awsMeta.Region, e.UseFIPS)
	case strings.HasPrefix(target, "dynamodb"):
		re, err = resolveDynamoDBEndpoint(req.Context(), awsMeta.Region, awsMeta.AccountID, e.UseFIPS)
	case strings.HasPrefix(target, "amazondax"):
		re, err = resolveDaxEndpoint(req.Context(), awsMeta.Region, e.UseFIPS)
	default:
		return nil, trace.BadParameter("DynamoDB API target %q is not recognized", target)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uri := e.sessionCtx.Database.GetURI()
	if uri != "" && uri != apiaws.DynamoDBURIForRegion(awsMeta.Region) {
		// Add a temporary schema to make a valid URL for url.Parse.
		if !strings.Contains(uri, "://") {
			uri = "schema://" + uri
		}
		u, err := url.Parse(uri)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// override the resolved endpoint URL with the user-configured URI.
		re.URL = u
	}
	// Force HTTPS
	re.URL.Scheme = "https"
	return re, nil
}

func resolveDynamoDBStreamsEndpoint(ctx context.Context, region string, useFIPS bool) (*endpoint, error) {
	params := dynamodbstreams.EndpointParameters{
		Region:  aws.String(region),
		UseFIPS: aws.Bool(useFIPS),
	}
	ep, err := dynamodbstreams.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &endpoint{
		URL:           &ep.URI,
		SigningRegion: region,
		// DynamoDB Streams uses the same signing name as DynamoDB.
		SigningName: "dynamodb",
	}, nil
}

func resolveDynamoDBEndpoint(ctx context.Context, region, accountID string, useFIPS bool) (*endpoint, error) {
	params := dynamodb.EndpointParameters{
		Region: aws.String(region),
		// Preferred means if we have an account ID available, then use an
		// account ID based endpoint.
		// We should always have the account ID available anyway.
		// If we didn't then it would just resolve the regional endpoint like
		// dynamodb.<region>.amazonaws.com.
		// AWS documents that account-based routing provides better request
		// performance for some services.
		// See: https://docs.aws.amazon.com/sdkref/latest/guide/feature-account-endpoints.html
		AccountIdEndpointMode: aws.String(aws.AccountIDEndpointModePreferred),
		UseFIPS:               aws.Bool(useFIPS),
	}
	if accountID != "" {
		params.AccountId = aws.String(accountID)
	}
	ep, err := dynamodb.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &endpoint{
		URL:           &ep.URI,
		SigningRegion: region,
		SigningName:   "dynamodb",
	}, nil
}

func resolveDaxEndpoint(ctx context.Context, region string, useFIPS bool) (*endpoint, error) {
	params := dax.EndpointParameters{
		Region:  aws.String(region),
		UseFIPS: aws.Bool(useFIPS),
	}
	ep, err := dax.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &endpoint{
		URL:           &ep.URI,
		SigningRegion: region,
		SigningName:   "dax",
	}, nil
}

// rewriteRequest clones a request, modifies the clone to rewrite its URL, and returns the modified request clone.
func rewriteRequest(ctx context.Context, r *http.Request, re *endpoint, body []byte) (*http.Request, error) {
	reqCopy := r.Clone(ctx)
	// set url and host header to match the resolved endpoint.
	reqCopy.URL = re.URL
	reqCopy.Host = re.URL.Host
	if body == nil {
		// no body is fine, skip copying it.
		return reqCopy, nil
	}

	// copy request body
	reqCopy.Body = io.NopCloser(bytes.NewReader(body))
	return reqCopy, nil
}

// getTargetHeader gets the X-Amz-Target header or returns an error if it is not
// present, as we rely on this header for endpoint resolution.
// See X-Amz-Target: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Programming.LowLevelAPI.html
func getTargetHeader(req *http.Request) (string, error) {
	target := req.Header.Get(libaws.AmzTargetHeader)
	if target == "" {
		return "", trace.BadParameter("missing %q header in http request", libaws.AmzTargetHeader)
	}
	return target, nil
}
