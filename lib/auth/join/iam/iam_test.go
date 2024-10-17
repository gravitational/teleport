// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package iam_test

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/join/iam"
	"github.com/gravitational/teleport/lib/utils/aws"
)

func TestCreateSignedSTSIdentityRequest(t *testing.T) {
	ctx := context.Background()

	t.Setenv("AWS_ACCESS_KEY_ID", "FAKE_KEY_ID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "FAKE_KEY")
	t.Setenv("AWS_SESSION_TOKEN", "FAKE_SESSION_TOKEN")

	const challenge = "asdf12345"

	for desc, tc := range map[string]struct {
		envRegion             string
		imdsRegion            string
		fips                  bool
		expectError           string
		expectEndpoint        string
		expectSignatureRegion string
	}{
		"no region": {
			expectEndpoint:        "sts.us-east-1.amazonaws.com",
			expectSignatureRegion: "us-east-1",
		},
		"no region fips": {
			fips:                  true,
			expectEndpoint:        "sts-fips.us-east-1.amazonaws.com",
			expectSignatureRegion: "us-east-1",
		},
		"us-west-2": {
			envRegion:             "us-west-2",
			expectEndpoint:        "sts.us-west-2.amazonaws.com",
			expectSignatureRegion: "us-west-2",
		},
		"us-west-2 with region from imdsv2": {
			imdsRegion:            "us-west-2",
			expectEndpoint:        "sts.us-west-2.amazonaws.com",
			expectSignatureRegion: "us-west-2",
		},
		"us-west-2 fips": {
			envRegion:             "us-west-2",
			fips:                  true,
			expectEndpoint:        "sts-fips.us-west-2.amazonaws.com",
			expectSignatureRegion: "us-west-2",
		},
		"us-west-2 fips with region from imdsv2": {
			imdsRegion:            "us-west-2",
			fips:                  true,
			expectEndpoint:        "sts-fips.us-west-2.amazonaws.com",
			expectSignatureRegion: "us-west-2",
		},
		"eu-central-1": {
			envRegion:             "eu-central-1",
			expectEndpoint:        "sts.eu-central-1.amazonaws.com",
			expectSignatureRegion: "eu-central-1",
		},
		"eu-central-1 fips": {
			envRegion: "eu-central-1",
			fips:      true,
			// All non-US regions have no FIPS endpoint and use the FIPS
			// endpoint in us-east-1.
			expectEndpoint:        "sts-fips.us-east-1.amazonaws.com",
			expectSignatureRegion: "us-east-1",
		},
		"ap-southeast-1": {
			envRegion:             "ap-southeast-1",
			expectEndpoint:        "sts.ap-southeast-1.amazonaws.com",
			expectSignatureRegion: "ap-southeast-1",
		},
		"ap-southeast-1 fips": {
			envRegion: "ap-southeast-1",
			fips:      true,
			// All non-US regions have no FIPS endpoint and try to use the FIPS
			// endpoint in us-east-1, but this will fail if the AWS credentials
			// were issued by the AWS China partition because they will not be
			// recognized by STS in the default partition. It will fail when
			// Auth sends the request to AWS, but this unit test only exercises
			// the client-side request generation.
			expectEndpoint:        "sts-fips.us-east-1.amazonaws.com",
			expectSignatureRegion: "us-east-1",
		},
		"govcloud": {
			envRegion:             "us-gov-east-1",
			expectEndpoint:        "sts.us-gov-east-1.amazonaws.com",
			expectSignatureRegion: "us-gov-east-1",
		},
		"govcloud fips": {
			envRegion: "us-gov-east-1",
			fips:      true,
			// All govcloud endpoints are FIPS.
			expectEndpoint:        "sts.us-gov-east-1.amazonaws.com",
			expectSignatureRegion: "us-gov-east-1",
		},
	} {
		t.Run(desc, func(t *testing.T) {
			if len(tc.envRegion) > 0 {
				t.Setenv("AWS_REGION", tc.envRegion)
			} else {
				// There's no t.Unsetenv so do this manually.
				prev := os.Getenv("AWS_REGION")
				os.Unsetenv("AWS_REGION")
				t.Cleanup(func() { os.Setenv("AWS_REGION", prev) })
			}

			imdsClient := &fakeIMDSClient{}
			if tc.imdsRegion != "" {
				imdsClient = &fakeIMDSClient{
					available: true,
					region:    tc.imdsRegion,
				}
			}

			// Create the signed sts:GetCallerIdentity request, which is a full
			// HTTP request with a body serialized into a byte slice.
			req, err := iam.CreateSignedSTSIdentityRequest(ctx, challenge,
				iam.WithFIPSEndpoint(tc.fips),
				iam.WithIMDSClient(imdsClient))
			if tc.expectError != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectError)
				return
			}
			require.NoError(t, err)

			// Parse the serialized HTTP request to check the endpoint and
			// parameters were correctly included by the AWS SDK.
			httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(req)))
			require.NoError(t, err)
			assert.Equal(t, tc.expectEndpoint, httpReq.Host)
			authHeader := httpReq.Header.Get(aws.AuthorizationHeader)
			sigV4, err := aws.ParseSigV4(authHeader)
			require.NoError(t, err)
			assert.Contains(t, sigV4.SignedHeaders, "x-teleport-challenge")
			assert.Equal(t, challenge, httpReq.Header.Get("x-teleport-challenge"))
			assert.Equal(t, tc.expectSignatureRegion, sigV4.Region, "signature region did not match expected")
		})
	}
}

type fakeIMDSClient struct {
	available bool
	region    string
}

func (c *fakeIMDSClient) IsAvailable(_ context.Context) bool {
	return c.available
}

func (c *fakeIMDSClient) GetRegion(_ context.Context) (string, error) {
	return c.region, nil
}
