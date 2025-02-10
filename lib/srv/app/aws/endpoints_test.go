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

	t.Run("unsupported SDK resolver", func(t *testing.T) {
		req, err := http.NewRequest("GET", "http://localhost", nil)
		require.NoError(t, err)

		_, err = signer.Sign(req, bytes.NewReader(nil), "ecr", "us-east-1", now)
		require.NoError(t, err)

		_, err = resolveEndpoint(req)
		require.Error(t, err)
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
