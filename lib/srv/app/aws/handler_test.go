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

package aws

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

type makeRequest func(url string, provider client.ConfigProvider, awsHost string) error

func s3Request(url string, provider client.ConfigProvider, awsHost string) error {
	return s3RequestWithTransport(url, provider, nil)
}

func s3RequestByAssumedRole(url string, provider client.ConfigProvider, awsHost string) error {
	return s3RequestWithTransport(url, provider, &requestByAssumedRoleTransport{xForwardedHost: awsHost})
}

func s3RequestWithTransport(url string, provider client.ConfigProvider, transport http.RoundTripper) error {
	s3Client := s3.New(provider, &aws.Config{
		Endpoint:   &url,
		MaxRetries: aws.Int(0),
		HTTPClient: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
	})
	_, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	return err
}

func dynamoRequest(url string, provider client.ConfigProvider, awsHost string) error {
	return dynamoRequestWithTransport(url, provider, nil)
}

func dynamoRequestByAssumedRole(url string, provider client.ConfigProvider, awsHost string) error {
	return dynamoRequestWithTransport(url, provider, &requestByAssumedRoleTransport{xForwardedHost: awsHost})
}

func dynamoRequestWithTransport(url string, provider client.ConfigProvider, transport http.RoundTripper) error {
	dynamoClient := dynamodb.New(provider, &aws.Config{
		Endpoint:   &url,
		MaxRetries: aws.Int(0),
		HTTPClient: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
	})
	_, err := dynamoClient.Scan(&dynamodb.ScanInput{
		TableName: aws.String("test-table"),
	})
	return err
}

// dont make tests generate huge requests just to test limiting the request
// size. Use a 1MB limit instead of the actual 70MB limit.
const maxTestHTTPRequestBodySize = 1 << 20

func maxSizeExceededRequest(url string, provider client.ConfigProvider, _ string) error {
	// fake an upload that's too large
	payload := strings.Repeat("x", maxTestHTTPRequestBodySize)
	return lambdaRequestWithPayload(url, provider, payload)
}

func lambdaRequest(url string, provider client.ConfigProvider, awsHost string) error {
	// fake a zip file with 70% of the max limit. Lambda will base64 encode it,
	// which bloats it up, and our proxy should still handle it.
	const size = (maxTestHTTPRequestBodySize * 7) / 10
	payload := strings.Repeat("x", size)
	return lambdaRequestWithPayload(url, provider, payload)
}

func lambdaRequestWithPayload(url string, provider client.ConfigProvider, payload string) error {
	lambdaClient := lambda.New(provider, &aws.Config{
		Endpoint:   &url,
		MaxRetries: aws.Int(0),
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	})
	_, err := lambdaClient.UpdateFunctionCode(&lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String("fakeFunc"),
		ZipFile:      []byte(payload),
	})
	return err
}

func assumeRoleRequest(requestDuration time.Duration) makeRequest {
	return func(url string, provider client.ConfigProvider, _ string) error {
		stsClient := stsutils.NewV1(provider, &aws.Config{
			Endpoint:   &url,
			MaxRetries: aws.Int(0),
			HTTPClient: &http.Client{
				Timeout: 5 * time.Second,
			},
		})

		_, err := stsClient.AssumeRole(&sts.AssumeRoleInput{
			DurationSeconds: aws.Int64(int64(requestDuration.Seconds())),
			RoleSessionName: aws.String("test-session"),
			RoleArn:         aws.String("arn:aws:iam::123456789012:role/test-role"),
		})
		return err
	}
}

type requestByAssumedRoleTransport struct {
	xForwardedHost string
}

func (r requestByAssumedRoleTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Simulate how a request by an assumed role is modified by "tsh".
	req.Host = r.xForwardedHost
	req.Header.Add("X-Forwarded-Host", r.xForwardedHost)
	req.Header.Add(common.TeleportAWSAssumedRole, fakeAssumedRoleARN)
	utils.RenameHeader(req.Header, awsutils.AuthorizationHeader, common.TeleportAWSAssumedRoleAuthorization)
	return http.DefaultTransport.RoundTrip(req)
}

func hasStatusCode(wantStatusCode int) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, msgAndArgs ...interface{}) {
		var apiErr awserr.RequestFailure
		require.ErrorAs(t, err, &apiErr, msgAndArgs...)
		require.Equal(t, wantStatusCode, apiErr.StatusCode(), msgAndArgs...)
	}
}

// TestAWSSignerHandler test the AWS SigningService APP handler logic with mocked STS signing credentials.
func TestAWSSignerHandler(t *testing.T) {
	consoleApp, err := types.NewAppV3(types.Metadata{
		Name: "awsconsole",
	}, types.AppSpecV3{
		URI:        constants.AWSConsoleURL,
		PublicAddr: "test.local",
	})
	require.NoError(t, err)

	consoleAppWithIntegration, err := types.NewAppV3(types.Metadata{
		Name: "awsconsole",
	}, types.AppSpecV3{
		URI:         constants.AWSConsoleURL,
		PublicAddr:  "test.local",
		Integration: "my-integration",
	})
	require.NoError(t, err)

	tests := []struct {
		name                string
		app                 types.Application
		awsClientSession    *session.Session
		awsSessionProvider  awsutils.AWSSessionProvider
		request             makeRequest
		advanceClock        time.Duration
		wantHost            string
		wantAuthCredService string
		wantAuthCredRegion  string
		wantAuthCredKeyID   string
		wantEventType       events.AuditEvent
		wantAssumedRole     string
		skipVerifySignature bool
		verifySentRequest   func(*testing.T, *http.Request)
		errAssertionFns     []require.ErrorAssertionFunc
	}{
		{
			name: "s3 access",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-west-2"),
			})),
			request:             s3Request,
			wantHost:            "s3.us-west-2.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "s3",
			wantAuthCredRegion:  "us-west-2",
			wantEventType:       &events.AppSessionRequest{},
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "s3 access with integration",
			app:  consoleAppWithIntegration,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-west-2"),
			})),
			request: s3Request,
			awsSessionProvider: func(ctx context.Context, region, integration string) (*session.Session, error) {
				if integration != "my-integration" {
					return nil, trace.BadParameter("")
				}
				return nil, nil
			},
			wantHost:            "s3.us-west-2.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "s3",
			wantAuthCredRegion:  "us-west-2",
			wantEventType:       &events.AppSessionRequest{},
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "s3 access with different region",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-west-1"),
			})),
			request:             s3Request,
			wantHost:            "s3.us-west-1.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "s3",
			wantAuthCredRegion:  "us-west-1",
			wantEventType:       &events.AppSessionRequest{},
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "s3 access missing credentials",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: credentials.AnonymousCredentials,
				Region:      aws.String("us-west-1"),
			})),
			request: s3Request,
			errAssertionFns: []require.ErrorAssertionFunc{
				hasStatusCode(http.StatusBadRequest),
			},
		},
		{
			name: "s3 access by assumed role",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForAssumedRole,
				Region:      aws.String("us-west-2"),
			})),
			request:             s3RequestByAssumedRole,
			wantHost:            "s3.us-west-2.amazonaws.com",
			wantAuthCredKeyID:   assumedRoleKeyID, // not using service's access key ID
			wantAuthCredService: "s3",
			wantAuthCredRegion:  "us-west-2",
			wantEventType:       &events.AppSessionRequest{},
			wantAssumedRole:     fakeAssumedRoleARN, // verifies assumed role is recorded in audit
			skipVerifySignature: true,               // not re-signing
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "DynamoDB access",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-east-1"),
			})),
			request:             dynamoRequest,
			wantHost:            "dynamodb.us-east-1.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "dynamodb",
			wantAuthCredRegion:  "us-east-1",
			wantEventType:       &events.AppSessionDynamoDBRequest{},
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "DynamoDB access with different region",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-west-1"),
			})),
			request:             dynamoRequest,
			wantHost:            "dynamodb.us-west-1.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "dynamodb",
			wantAuthCredRegion:  "us-west-1",
			wantEventType:       &events.AppSessionDynamoDBRequest{},
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "DynamoDB access missing credentials",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: credentials.AnonymousCredentials,
				Region:      aws.String("us-west-1"),
			})),
			request: dynamoRequest,
			errAssertionFns: []require.ErrorAssertionFunc{
				hasStatusCode(http.StatusBadRequest),
			},
		},
		{
			name: "DynamoDB access by assumed role",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForAssumedRole,
				Region:      aws.String("us-east-1"),
			})),
			request:             dynamoRequestByAssumedRole,
			wantHost:            "dynamodb.us-east-1.amazonaws.com",
			wantAuthCredKeyID:   assumedRoleKeyID, // not using service's access key ID
			wantAuthCredService: "dynamodb",
			wantAuthCredRegion:  "us-east-1",
			wantEventType:       &events.AppSessionDynamoDBRequest{},
			wantAssumedRole:     fakeAssumedRoleARN, // verifies assumed role is recorded in audit
			skipVerifySignature: true,               // not re-signing
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "Lambda access",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-east-1"),
			})),
			request:             lambdaRequest,
			wantHost:            "lambda.us-east-1.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "lambda",
			wantAuthCredRegion:  "us-east-1",
			wantEventType:       &events.AppSessionRequest{},
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "Request exceeding max size",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-east-1"),
			})),
			request: maxSizeExceededRequest,
			errAssertionFns: []require.ErrorAssertionFunc{
				// TODO(gavin): change this to [http.StatusRequestEntityTooLarge]
				// after updating [trace.ErrorToCode].
				hasStatusCode(http.StatusTooManyRequests),
			},
		},
		{
			name: "AssumeRole success (shorter identity duration)",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-east-1"),
			})),
			request:             assumeRoleRequest(2 * time.Hour),
			advanceClock:        10 * time.Minute,
			wantHost:            "sts.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "sts",
			wantAuthCredRegion:  "us-east-1",
			wantEventType:       &events.AppSessionRequest{},
			verifySentRequest:   verifyAssumeRoleDuration(50 * time.Minute), // 1h (suite default for identity) - 10m
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "AssumeRole success (shorter requested duration)",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-east-1"),
			})),
			request:             assumeRoleRequest(32 * time.Minute),
			wantHost:            "sts.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "sts",
			wantAuthCredRegion:  "us-east-1",
			wantEventType:       &events.AppSessionRequest{},
			verifySentRequest:   verifyAssumeRoleDuration(32 * time.Minute), // matches the request
			errAssertionFns: []require.ErrorAssertionFunc{
				require.NoError,
			},
		},
		{
			name: "AssumeRole denied",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: staticAWSCredentialsForClient,
				Region:      aws.String("us-east-1"),
			})),
			request:             assumeRoleRequest(2 * time.Hour),
			advanceClock:        50 * time.Minute, // identity is expiring in 10m which is less than minimum
			wantHost:            "sts.amazonaws.com",
			wantAuthCredKeyID:   "AKIDl",
			wantAuthCredService: "sts",
			wantAuthCredRegion:  "us-east-1",
			wantEventType:       &events.AppSessionRequest{},
			errAssertionFns: []require.ErrorAssertionFunc{
				hasStatusCode(http.StatusForbidden),
			},
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fakeClock := clockwork.NewFakeClock()
			mockAwsHandler := func(w http.ResponseWriter, r *http.Request) {
				// check that we got what the test case expects first.
				assert.Equal(t, tc.wantHost, r.Host)
				awsAuthHeader, err := awsutils.ParseSigV4(r.Header.Get(awsutils.AuthorizationHeader))
				if !assert.NoError(t, err) {
					http.Error(w, err.Error(), trace.ErrorToCode(err))
					return
				}
				assert.Equal(t, tc.wantAuthCredRegion, awsAuthHeader.Region)
				assert.Equal(t, tc.wantAuthCredKeyID, awsAuthHeader.KeyID)
				assert.Equal(t, tc.wantAuthCredService, awsAuthHeader.Service)

				// check that the signature is valid.
				if !tc.skipVerifySignature {
					err = awsutils.VerifyAWSSignature(r, staticAWSCredentials)
					if !assert.NoError(t, err) {
						http.Error(w, err.Error(), trace.ErrorToCode(err))
						return
					}
				}
				// extra checks
				if tc.verifySentRequest != nil {
					tc.verifySentRequest(t, r)
				}

				w.WriteHeader(http.StatusOK)
			}

			sessionProvider := awsutils.SessionProviderUsingAmbientCredentials()
			if tc.awsSessionProvider != nil {
				sessionProvider = tc.awsSessionProvider
			}

			suite := createSuite(t, mockAwsHandler, tc.app, fakeClock, sessionProvider)
			fakeClock.Advance(tc.advanceClock)

			err := tc.request(suite.URL, tc.awsClientSession, tc.wantHost)
			for _, assertFn := range tc.errAssertionFns {
				assertFn(t, err)
			}

			// Validate audit event.
			if err == nil {
				require.Len(t, suite.recorder.C(), 1)

				event := <-suite.recorder.C()
				switch appSessionEvent := event.(type) {
				case *events.AppSessionDynamoDBRequest:
					_, ok := tc.wantEventType.(*events.AppSessionDynamoDBRequest)
					require.True(t, ok, "unexpected event type: wanted %T but got %T", tc.wantEventType, appSessionEvent)
					require.Equal(t, tc.wantHost, appSessionEvent.AWSHost)
					require.Equal(t, tc.wantAuthCredService, appSessionEvent.AWSService)
					require.Equal(t, tc.wantAuthCredRegion, appSessionEvent.AWSRegion)
					require.Equal(t, tc.wantAssumedRole, appSessionEvent.AWSAssumedRole)
					j, err := appSessionEvent.Body.MarshalJSON()
					require.NoError(t, err)
					require.Empty(t, cmp.Diff(`{"TableName":"test-table"}`, string(j)))
				case *events.AppSessionRequest:
					_, ok := tc.wantEventType.(*events.AppSessionRequest)
					require.True(t, ok, "unexpected event type: wanted %T but got %T", tc.wantEventType, appSessionEvent)
					require.Equal(t, tc.wantHost, appSessionEvent.AWSHost)
					require.Equal(t, tc.wantAuthCredService, appSessionEvent.AWSService)
					require.Equal(t, tc.wantAuthCredRegion, appSessionEvent.AWSRegion)
					require.Equal(t, tc.wantAssumedRole, appSessionEvent.AWSAssumedRole)
				default:
					require.FailNow(t, "wrong event type", "unexpected event type: wanted %T but got %T", tc.wantEventType, appSessionEvent)
				}
			} else {
				require.Empty(t, suite.recorder.C())
			}
		})
	}
}

func TestRewriteRequest(t *testing.T) {
	expectedReq, err := http.NewRequest("GET", "https://example.com", http.NoBody)
	require.NoError(t, err)
	ctx := context.Background()

	inputReq := mustNewRequest(t, "GET", "https://example.com", nil)
	actualOutReq, err := rewriteRequest(ctx, inputReq, &endpoints.ResolvedEndpoint{})
	require.NoError(t, err)
	require.Equal(t, expectedReq, actualOutReq, err)

	_, err = io.ReadAll(actualOutReq.Body)
	require.NoError(t, err)
}

func TestURLForResolvedEndpoint(t *testing.T) {
	tests := []struct {
		name                 string
		inputReq             *http.Request
		inputResolvedEnpoint *endpoints.ResolvedEndpoint
		requireError         require.ErrorAssertionFunc
		expectURL            *url.URL
	}{
		{
			name:     "bad resolved endpoint",
			inputReq: mustNewRequest(t, "GET", "http://1.2.3.4/hello/world?aa=2", nil),
			inputResolvedEnpoint: &endpoints.ResolvedEndpoint{
				URL: string([]byte{0x05}),
			},
			requireError: require.Error,
		},
		{
			name:     "replaced host and scheme",
			inputReq: mustNewRequest(t, "GET", "http://1.2.3.4/hello/world?aa=2", nil),
			inputResolvedEnpoint: &endpoints.ResolvedEndpoint{
				URL: "https://local.test.com",
			},
			expectURL: &url.URL{
				Scheme:   "https",
				Host:     "local.test.com",
				Path:     "/hello/world",
				RawQuery: "aa=2",
			},
			requireError: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualURL, err := urlForResolvedEndpoint(test.inputReq, test.inputResolvedEnpoint)
			require.Equal(t, test.expectURL, actualURL)
			test.requireError(t, err)
		})
	}
}

func mustNewRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	t.Helper()

	r, err := http.NewRequest(method, url, body)
	require.NoError(t, err)
	return r
}

const assumedRoleKeyID = "assumedRoleKeyID"

var (
	staticAWSCredentialsForAssumedRole = credentials.NewStaticCredentials(assumedRoleKeyID, "assumedRoleKeySecret", "")
	staticAWSCredentials               = credentials.NewStaticCredentials("AKIDl", "SECRET", "SESSION")
	staticAWSCredentialsForClient      = credentials.NewStaticCredentials("fakeClientKeyID", "fakeClientSecret", "")
)

type suite struct {
	*httptest.Server
	identity *tlsca.Identity
	app      types.Application
	recorder *eventstest.ChannelRecorder
}

func createSuite(t *testing.T, mockAWSHandler http.HandlerFunc, app types.Application, clock clockwork.Clock, awsSessionProvider awsutils.AWSSessionProvider) *suite {
	recorder := eventstest.NewChannelRecorder(1)
	identity := tlsca.Identity{
		Username: "user",
		Expires:  clock.Now().Add(time.Hour),
		RouteToApp: tlsca.RouteToApp{
			AWSRoleARN: "arn:aws:iam::123456789012:role/test",
		},
	}

	awsAPIMock := httptest.NewUnstartedServer(mockAWSHandler)
	awsAPIMock.StartTLS()
	t.Cleanup(func() {
		awsAPIMock.Close()
	})

	svc, err := awsutils.NewSigningService(awsutils.SigningServiceConfig{
		SessionProvider:   awsSessionProvider,
		CredentialsGetter: awsutils.NewStaticCredentialsGetter(staticAWSCredentials),
		Clock:             clock,
	})
	require.NoError(t, err)

	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  libevents.NewDiscardEmitter(),
		Recorder: libevents.WithNoOpPreparer(recorder),
	})
	require.NoError(t, err)
	signerHandler, err := NewAWSSignerHandler(context.Background(),
		SignerHandlerConfig{
			SigningService: svc,
			RoundTripper: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return net.Dial(awsAPIMock.Listener.Addr().Network(), awsAPIMock.Listener.Addr().String())
				},
			},
			Clock:                  clock,
			MaxHTTPRequestBodySize: maxTestHTTPRequestBodySize,
		})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		request = common.WithSessionContext(request, &common.SessionContext{
			Identity: &identity,
			App:      app,
			Audit:    audit,
			ChunkID:  "123abc",
		})

		signerHandler.ServeHTTP(writer, request)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.Close()
	})

	return &suite{
		Server:   server,
		identity: &identity,
		app:      app,
		recorder: recorder,
	}
}

func verifyAssumeRoleDuration(wantDuration time.Duration) func(*testing.T, *http.Request) {
	return func(t *testing.T, req *http.Request) {
		clone, err := cloneRequest(req)
		require.NoError(t, err)
		require.NoError(t, clone.ParseForm())
		require.Equal(t, wantDuration, getAssumeRoleQueryDuration(clone.PostForm))
	}
}

const fakeAssumedRoleARN = "arn:aws:sts::123456789012:assumed-role/role-name/role-session-name"
