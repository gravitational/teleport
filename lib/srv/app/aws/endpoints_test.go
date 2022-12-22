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

package aws

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/require"
)

func TestResolveEndpoints(t *testing.T) {
	signer := v4.NewSigner(credentials.NewStaticCredentials("fakeClientKeyID", "fakeClientSecret", ""))
	region := "us-east-1"
	now := time.Now()

	t.Run("AWS SDK resolver", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost", nil)
		require.NoError(t, err)

		_, err = signer.Sign(req, bytes.NewReader(nil), "ecr", "us-east-1", now)
		require.NoError(t, err)

		endpoint, err := resolveEndpoint(req)
		require.NoError(t, err)
		require.Equal(t, "ecr", endpoint.SigningName)
		require.Equal(t, "https://api.ecr.us-east-1.amazonaws.com", endpoint.URL)
	})

	t.Run("X-Forwarded-Host", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-Host", "some-service.us-east-1.amazonaws.com")

		_, err = signer.Sign(req, bytes.NewReader(nil), "some-service", region, now)
		require.NoError(t, err)

		endpoint, err := resolveEndpoint(req)
		require.NoError(t, err)
		require.Equal(t, "some-service", endpoint.SigningName)
		require.Equal(t, "https://some-service.us-east-1.amazonaws.com", endpoint.URL)
	})
}
