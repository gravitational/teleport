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
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/stretchr/testify/require"

	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

func TestResolveEndpoints(t *testing.T) {
	creds := aws.Credentials{AccessKeyID: "fakeClientKeyID", SecretAccessKey: "fakeClientSecret"}
	signer := v4.NewSigner()
	region := "us-east-1"
	now := time.Now()

	t.Run("unsupported SDK resolver", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost", nil)
		require.NoError(t, err)

		err = signer.SignHTTP(t.Context(), creds, req, awsutils.EmptyPayloadHash, "ecr", "us-east-1", now)
		require.NoError(t, err)

		_, err = resolveEndpoint(req, awsutils.AuthorizationHeader)
		require.Error(t, err)
	})

	t.Run("X-Forwarded-Host", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-Host", "some-service.us-east-1.amazonaws.com")

		err = signer.SignHTTP(t.Context(), creds, req, awsutils.EmptyPayloadHash, "some-service", region, now)
		require.NoError(t, err)

		endpoint, err := resolveEndpoint(req, awsutils.AuthorizationHeader)
		require.NoError(t, err)
		require.Equal(t, "some-service", endpoint.SigningName)
		require.Equal(t, "https://some-service.us-east-1.amazonaws.com", endpoint.URL)
	})
}
