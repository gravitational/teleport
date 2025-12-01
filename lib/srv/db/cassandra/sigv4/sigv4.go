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
	"time"

	"github.com/gocql/gocql"
	"github.com/gravitational/trace"
)

// Note that this file is copied from the original lib with some minor
// adjustments like removing unused AWS session and callback:
// https://github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/blob/main/sigv4/sigv4.go

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
