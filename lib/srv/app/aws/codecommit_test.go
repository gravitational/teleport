/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	libawsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/stretchr/testify/require"
)

func Test_codeCommitSigV4Signer(t *testing.T) {
	date := "20220222T222222Z"
	credValue := credentials.Value{
		AccessKeyID:     "example-access-key-id",
		SecretAccessKey: "example-secret-access-key",
	}
	signTime, err := libawsutils.ParseAmazonDateTime(date)
	require.NoError(t, err)

	signer := &codeCommitSigV4Signer{
		region:    "ca-central-1",
		hostname:  "git-codecommit.ca-central-1.amazonaws.com",
		path:      "my-repo",
		signTime:  signTime,
		credValue: credValue,
	}

	// All expected values are calculated from using botocore lib:
	// https://github.com/aws/git-remote-codecommit/blob/master/git_remote_codecommit/__init__.py
	expectedCanonicalRequest := `GIT
my-repo

host:git-codecommit.ca-central-1.amazonaws.com

host
`
	require.Equal(t, expectedCanonicalRequest, signer.canonicalRequest())

	expectedStringToSign := `AWS4-HMAC-SHA256
20220222T222222
20220222/ca-central-1/codecommit/aws4_request
d12655b601b13f5eb8eb058aa746e812881301ed96124748df87735142777fdf`
	actualStringToSign, err := signer.stringToSign()
	require.NoError(t, err)
	require.Equal(t, expectedStringToSign, actualStringToSign)

	expectedSignature := "46d2b6a9501b2738642bf0b2787de2397d2407d12cb55b3e6f61d7a169ca2872"
	actualSignature, err := signer.signature()
	require.NoError(t, err)
	require.Equal(t, expectedSignature, actualSignature)
}
