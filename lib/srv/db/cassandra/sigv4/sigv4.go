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
	"time"

	"github.com/gocql/gocql"
	"github.com/gravitational/trace"
)

// Note that this file is copied from the original lib with some minor
// adjustments like removing unused AWS session and callback:
// https://github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/blob/3de2214b893e307a7447a754489e4d7dfdf6d0c0/sigv4/sigv4.go

// AwsAuthenticator implements gocql.Authenticator for AWS Keyspaces.
type AwsAuthenticator struct {
	Region          string
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
	currentTime     time.Time
}

func (a AwsAuthenticator) Challenge(req []byte) ([]byte, gocql.Authenticator, error) {
	// copy these rather than use a reference due to how gocql creates connections (it's just
	// safer if everything is a fresh copy).
	auth := signingAuthenticator{
		region:          a.Region,
		accessKeyId:     a.AccessKeyId,
		secretAccessKey: a.SecretAccessKey,
		sessionToken:    a.SessionToken,
		currentTime:     a.currentTime,
	}
	return []byte("SigV4\000\000"), auth, nil
}

func (a AwsAuthenticator) Success([]byte) error {
	return nil
}

// signingAuthenticator is the internal private authenticator we actually use
type signingAuthenticator struct {
	region          string
	accessKeyId     string
	secretAccessKey string
	sessionToken    string

	// currentTime is mainly used for testing and not exposed
	currentTime time.Time
}

func (p signingAuthenticator) Challenge(req []byte) ([]byte, gocql.Authenticator, error) {
	nonce, err := extractNonce(req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// init the time if not provided.
	t := p.currentTime
	if t.IsZero() {
		t = time.Now().UTC()
	}

	accessKeyId := p.accessKeyId
	secretAccessKey := p.secretAccessKey
	sessionToken := p.sessionToken

	signedResponse := buildSignedResponse(
		p.region,
		nonce,
		accessKeyId,
		secretAccessKey,
		sessionToken,
		t,
	)

	// copy this to a separate byte array to prevent some slicing corruption with how the framer object works
	resp := make([]byte, len(signedResponse))
	copy(resp, signedResponse)

	return resp, nil, nil
}

func (p signingAuthenticator) Success([]byte) error {
	return nil
}
