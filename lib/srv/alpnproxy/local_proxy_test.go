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

package alpnproxy

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// TestHandleAWSAccessSigVerification tests if LocalProxy verifies the AWS SigV4 signature of incoming request.
func TestHandleAWSAccessSigVerification(t *testing.T) {
	var (
		firstAWSCred  = credentials.NewStaticCredentials("userID", "firstSecret", "")
		secondAWSCred = credentials.NewStaticCredentials("userID", "secondSecret", "")

		awsService = "s3"
		awsRegion  = "eu-central-1"
	)

	testCases := []struct {
		name       string
		originCred *credentials.Credentials
		proxyCred  *credentials.Credentials
		wantErr    require.ErrorAssertionFunc
		wantStatus int
	}{
		{
			name:       "valid signature",
			originCred: firstAWSCred,
			proxyCred:  firstAWSCred,
			wantErr:    require.NoError,
			wantStatus: http.StatusOK,
		},
		{
			name:       "different aws credential",
			originCred: firstAWSCred,
			proxyCred:  secondAWSCred,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "unsigned request",
			originCred: nil,
			proxyCred:  firstAWSCred,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lp := createAWSAccessProxySuite(t, tc.proxyCred)

			url := url.URL{
				Scheme: "http",
				Host:   lp.GetAddr(),
				Path:   "/",
			}

			pr := bytes.NewReader([]byte("payload content"))
			req, err := http.NewRequest(http.MethodGet, url.String(), pr)
			require.NoError(t, err)

			if tc.originCred != nil {
				v4.NewSigner(tc.originCred).Sign(req, pr, awsService, awsRegion, time.Now())
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
	}
}

// Verifies s3 requests are signed without URL escaping to match AWS SDKs.
func TestHandleAWSAccessS3Signing(t *testing.T) {
	cred := credentials.NewStaticCredentials("access-key", "secret-key", "")
	lp := createAWSAccessProxySuite(t, cred)

	// Avoid loading extra things.
	t.Setenv("AWS_SDK_LOAD_CONFIG", "false")

	// Create a real AWS SDK s3 client.
	awsConfig := aws.NewConfig().
		WithDisableSSL(true).
		WithRegion("local").
		WithCredentials(cred).
		WithEndpoint(lp.GetAddr()).
		WithS3ForcePathStyle(true)

	s3client := s3.New(session.Must(session.NewSession(awsConfig)))

	// Use a bucket name with special charaters. AWS SDK actually signs the
	// request with the unescaped bucket name.
	_, err := s3client.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String("=bucket=name="),
	})

	// Our signature verification should succeed to match what AWS SDK signs.
	require.NoError(t, err)
}

func createAWSAccessProxySuite(t *testing.T, cred *credentials.Credentials) *LocalProxy {
	hs := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {}))
	listener := mustCreateListener(t)
	t.Cleanup(func() {
		listener.Close()
	})

	lp, err := NewLocalProxy(LocalProxyConfig{
		Listener:           listener,
		RemoteProxyAddr:    hs.Listener.Addr().String(),
		Protocol:           common.ProtocolHTTP,
		ParentContext:      context.Background(),
		InsecureSkipVerify: true,
		AWSCredentials:     cred,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := lp.Close()
		require.NoError(t, err)
	})
	go func() {
		err := lp.StartAWSAccessProxy(context.Background())
		require.NoError(t, err)
	}()
	return lp
}

func mustCreateListener(t *testing.T) net.Listener {
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	return listener
}
