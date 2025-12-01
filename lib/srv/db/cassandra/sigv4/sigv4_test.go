/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package sigv4

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Note that all of these tests are copied from the original library:
// https://github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/blob/main/sigv4/sigv4_test.go

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
