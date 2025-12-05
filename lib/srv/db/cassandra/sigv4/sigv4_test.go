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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Note that all of these tests are copied from the original library:
// https://github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/blob/3de2214b893e307a7447a754489e4d7dfdf6d0c0/sigv4/sigv4_test.go

// We should switch to sigv4 when initially challenged
func TestShouldReturnSigV4iInitially(t *testing.T) {
	target := AwsAuthenticator{}
	resp, _, _ := target.Challenge(nil)

	assert.Equal(t, "SigV4\000\000", string(resp))
}

func TestShouldTranslate(t *testing.T) {
	target := buildStdTarget()
	_, challenger, _ := target.Challenge(nil)

	stdNonce := []byte("nonce=91703fdc2ef562e19fbdab0f58e42fe5")
	resp, _, _ := challenger.Challenge(stdNonce)
	expected := "signature=7f3691c18a81b8ce7457699effbfae5b09b4e0714ab38c1292dbdf082c9ddd87,access_key=UserID-1,amzdate=2020-06-09T22:41:51.000Z"
	assert.Equal(t, expected, string(resp))
}

func buildStdTarget() *AwsAuthenticator {
	target := AwsAuthenticator{
		Region:          "us-west-2",
		AccessKeyId:     "UserID-1",
		SecretAccessKey: "UserSecretKey-1",
	}
	target.currentTime, _ = time.Parse(time.RFC3339, "2020-06-09T22:41:51Z")
	return &target
}
