/*
Copyright 2021 Gravitational, Inc.

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

package aws

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

type makeRequest func(url string, provider client.ConfigProvider) error

func s3Request(url string, provider client.ConfigProvider) error {
	s3Client := s3.New(provider, &aws.Config{
		Endpoint: &url,
	})
	_, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	return err
}

func dynamoRequest(url string, provider client.ConfigProvider) error {
	dynamoClient := dynamodb.New(provider, &aws.Config{
		Endpoint: &url,
	})
	_, err := dynamoClient.Scan(&dynamodb.ScanInput{
		TableName: aws.String("test-table"),
	})
	return err
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

	tests := []struct {
		name                string
		app                 types.Application
		awsClientSession    *session.Session
		request             makeRequest
		wantHost            string
		wantAuthCredService string
		wantAuthCredRegion  string
		wantAuthCredKeyID   string
		wantEventType       events.AuditEvent
		errAssertionFns     []require.ErrorAssertionFunc
	}{
		{
			name: "s3 access",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
					AccessKeyID:     "fakeClientKeyID",
					SecretAccessKey: "fakeClientSecret",
				}}),
				Region: aws.String("us-west-2"),
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
			name: "s3 access with different region",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
					AccessKeyID:     "fakeClientKeyID",
					SecretAccessKey: "fakeClientSecret",
				}}),
				Region: aws.String("us-west-1"),
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
			name: "DynamoDB access",
			app:  consoleApp,
			awsClientSession: session.Must(session.NewSession(&aws.Config{
				Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
					AccessKeyID:     "fakeClientKeyID",
					SecretAccessKey: "fakeClientSecret",
				}}),
				Region: aws.String("us-east-1"),
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
				Credentials: credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
					AccessKeyID:     "fakeClientKeyID",
					SecretAccessKey: "fakeClientSecret",
				}}),
				Region: aws.String("us-west-1"),
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := func(writer http.ResponseWriter, request *http.Request) {
				require.Equal(t, tc.wantHost, request.Host)
				awsAuthHeader, err := awsutils.ParseSigV4(request.Header.Get(awsutils.AuthorizationHeader))
				require.NoError(t, err)
				require.Equal(t, tc.wantAuthCredRegion, awsAuthHeader.Region)
				require.Equal(t, tc.wantAuthCredKeyID, awsAuthHeader.KeyID)
				require.Equal(t, tc.wantAuthCredService, awsAuthHeader.Service)
			}

			suite := createSuite(t, handler, tc.app)

			err := tc.request(suite.URL, tc.awsClientSession)
			for _, assertFn := range tc.errAssertionFns {
				assertFn(t, err)
			}

			// Validate audit event.
			if err == nil {
				require.Len(t, suite.emitter.C(), 1)

				event := <-suite.emitter.C()
				switch appSessionEvent := event.(type) {
				case *events.AppSessionDynamoDBRequest:
					_, ok := tc.wantEventType.(*events.AppSessionDynamoDBRequest)
					require.True(t, ok, "unexpected event type: wanted %T but got %T", tc.wantEventType, appSessionEvent)
					require.Equal(t, tc.wantHost, appSessionEvent.AWSHost)
					require.Equal(t, tc.wantAuthCredService, appSessionEvent.AWSService)
					require.Equal(t, tc.wantAuthCredRegion, appSessionEvent.AWSRegion)
					j, err := appSessionEvent.Body.MarshalJSON()
					require.NoError(t, err)
					require.Empty(t, cmp.Diff(`{"TableName":"test-table"}`, string(j)))
				case *events.AppSessionRequest:
					_, ok := tc.wantEventType.(*events.AppSessionRequest)
					require.True(t, ok, "unexpected event type: wanted %T but got %T", tc.wantEventType, appSessionEvent)
					require.Equal(t, tc.wantHost, appSessionEvent.AWSHost)
					require.Equal(t, tc.wantAuthCredService, appSessionEvent.AWSService)
					require.Equal(t, tc.wantAuthCredRegion, appSessionEvent.AWSRegion)
				default:
					require.FailNow(t, "wrong event type", "unexpected event type: wanted %T but got %T", tc.wantEventType, appSessionEvent)
				}
			} else {
				require.Len(t, suite.emitter.C(), 0)
			}
		})
	}
}

func TestURLForResolvedEndpoint(t *testing.T) {
	tests := []struct {
		name                 string
		inputReq             *http.Request
		inputResolvedEnpoint *endpoints.ResolvedEndpoint
		requireError         require.ErrorAssertionFunc
		expectURL            string
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
			expectURL:    "https://local.test.com/hello/world?aa=2",
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

func staticAWSCredentials(client.ConfigProvider, *common.SessionContext) *credentials.Credentials {
	return credentials.NewStaticCredentials("AKIDl", "SECRET", "SESSION")
}

type suite struct {
	*httptest.Server
	identity *tlsca.Identity
	app      types.Application
	emitter  *eventstest.ChannelEmitter
}

func createSuite(t *testing.T, handler http.HandlerFunc, app types.Application) *suite {
	emitter := eventstest.NewChannelEmitter(1)
	user := auth.LocalUser{Username: "user"}

	awsAPIMock := httptest.NewUnstartedServer(handler)
	awsAPIMock.StartTLS()
	t.Cleanup(func() {
		awsAPIMock.Close()
	})

	svc, err := NewSigningService(SigningServiceConfig{
		getSigningCredentials: staticAWSCredentials,
		RoundTripper: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial(awsAPIMock.Listener.Addr().Network(), awsAPIMock.Listener.Addr().String())
			},
		},
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	audit, err := common.NewAudit(common.AuditConfig{
		Emitter: emitter,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		request = common.WithSessionContext(request, &common.SessionContext{
			Identity: &user.Identity,
			App:      app,
			Audit:    audit,
		})

		svc.ServeHTTP(writer, request)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.Close()
	})

	return &suite{
		Server:   server,
		identity: &user.Identity,
		app:      app,
		emitter:  emitter,
	}
}
