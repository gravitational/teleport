/*
 *  Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License").
 *  You may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *   Unless required by applicable law or agreed to in writing, software
 *   distributed under the License is distributed on an "AS IS" BASIS,
 *   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *   See the License for the specific language governing permissions and
 *   limitations under the License.
 */

package sigv4

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Note that all of these tests are copied from the original library:
// https://github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/blob/3de2214b893e307a7447a754489e4d7dfdf6d0c0/sigv4/internal/crypt_test.go

const (
	nonce       = "91703fdc2ef562e19fbdab0f58e42fe5"
	region      = "us-west-2"
	accessKeyId = "UserID-1"
	secret      = "UserSecretKey-1"
)

// buildStdInstant produces arbitrary time 2020-06-09T22:41:51Z
func buildStdInstant() time.Time {
	result, _ := time.Parse(time.RFC3339, "2020-06-09T22:41:51Z")
	return result
}

// TestExtractNonceSuccess tests we should switch to sigv4 when initially
// challenged.
func TestExtractNonceSuccess(t *testing.T) {
	challenge := []byte("nonce=1256")
	actualNonce, _ := extractNonce(challenge)
	assert.Equal(t, "1256", actualNonce)
}

func TestExtractNonceMissing(t *testing.T) {
	challenge := []byte("n1256")
	_, err := extractNonce(challenge)
	assert.Error(t, err)
}

func TestComputeScope(t *testing.T) {
	scope := computeScope(buildStdInstant(), "us-west-2")
	assert.Equal(t, "20200609/us-west-2/cassandra/aws4_request", scope)
}

func TestFormCanonicalRequest(t *testing.T) {
	scope := "20200609/us-west-2/cassandra/aws4_request"
	canonicalRequest := "PUT\n" +
		"/authenticate\n" +
		"X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=UserID-1%2F20200609%2Fus-west-2%2Fcassandra%2Faws4_request&X-Amz-Date=2020-06-09T22%3A41%3A51.000Z&X-Amz-Expires=900\n" +
		"host:cassandra\n\n" +
		"host\n" +
		"ddf250111597b3f35e51e649f59e3f8b30ff5b247166d709dc1b1e60bd927070"

	actual := formCanonicalRequest("UserID-1", scope, buildStdInstant(), nonce)
	assert.Equal(t, canonicalRequest, actual)
}

func TestDeriveSigningKey(t *testing.T) {
	expected := "7fb139473f153aec1b05747b0cd5cd77a1186d22ae895a3a0128e699d72e1aba"

	actual := deriveSigningKey(secret, buildStdInstant(), region)
	assert.Equal(t, expected, hex.EncodeToString(actual))
}

func TestCreateSignature(t *testing.T) {
	signingKey, _ := hex.DecodeString("7fb139473f153aec1b05747b0cd5cd77a1186d22ae895a3a0128e699d72e1aba")
	scope := "20200609/us-west-2/cassandra/aws4_request"
	canonicalRequest := "PUT\n" +
		"/authenticate\n" +
		"X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=UserID-1%2F20200609%2Fus-west-2%2Fcassandra%2Faws4_request&X-Amz-Date=2020-06-09T22%3A41%3A51.000Z&X-Amz-Expires=900\n" +
		"host:cassandra\n\n" +
		"host\n" +
		"ddf250111597b3f35e51e649f59e3f8b30ff5b247166d709dc1b1e60bd927070"

	actual := createSignature(canonicalRequest, buildStdInstant(), scope, signingKey)
	expected := "7f3691c18a81b8ce7457699effbfae5b09b4e0714ab38c1292dbdf082c9ddd87"
	assert.Equal(t, expected, hex.EncodeToString(actual))
}

func TestBuildSignedResponse(t *testing.T) {
	actual := buildSignedResponse(region, nonce, accessKeyId, secret, "", buildStdInstant())
	expected := "signature=7f3691c18a81b8ce7457699effbfae5b09b4e0714ab38c1292dbdf082c9ddd87,access_key=UserID-1,amzdate=2020-06-09T22:41:51.000Z"
	assert.Equal(t, expected, actual)
}

func TestBuildSignedResponseWithSessionToken(t *testing.T) {
	sessionToken := "sess-token-1"
	actual := buildSignedResponse(region, nonce, accessKeyId, secret, sessionToken, buildStdInstant())
	expected := "signature=7f3691c18a81b8ce7457699effbfae5b09b4e0714ab38c1292dbdf082c9ddd87,access_key=UserID-1,amzdate=2020-06-09T22:41:51.000Z,session_token=sess-token-1"
	assert.Equal(t, expected, actual)
}
