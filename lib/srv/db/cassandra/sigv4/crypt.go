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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// Note that this file is copied from the original lib:
// https://github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin/blob/main/sigv4/internal/crypt.go

// extractNonce extracts the nonce from a request payload needed for calls from
// payload returned by Amazon Keyspaces.
func extractNonce(req []byte) (string, error) {
	text := string(req)
	if !strings.HasPrefix(text, "nonce=") {
		return "", trace.Errorf("request does not contain nonce property")
	}
	nonce := strings.Split(text, "nonce=")[1]
	return nonce, nil
}

// toCredDateStamp converts time to an aws credential timestamp
// such as 2020-06-09T22:41:51.000Z -> '20200609'
func toCredDateStamp(t time.Time) string {
	return fmt.Sprintf("%d%02d%02d", t.Year(), t.Month(), t.Day())
}

// computeScope computes the scope to be used in the request
func computeScope(t time.Time, region string) string {
	a := []string{
		toCredDateStamp(t),
		region,
		"cassandra",
		"aws4_request"}
	return strings.Join(a, "/")
}

func formCanonicalRequest(accessKeyId string, scope string, t time.Time, nonce string) string {
	nonceHash := sha256.Sum256([]byte(nonce))
	headers := []string{
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		fmt.Sprintf("X-Amz-Credential=%s%%2F%s", accessKeyId, url.QueryEscape(scope)),
		fmt.Sprintf("X-Amz-Date=%s", url.QueryEscape(t.Format("2006-01-02T15:04:05.000Z"))),
		"X-Amz-Expires=900"}
	sort.Strings(headers)
	queryString := strings.Join(headers, "&")

	return fmt.Sprintf("PUT\n/authenticate\n%s\nhost:cassandra\n\nhost\n%s", queryString, hex.EncodeToString(nonceHash[:]))
}

// applyHmac applies hmac with given string
// useful as our protocol requires lots of iterative hmacs
func applyHmac(data string, hashSecret []byte) []byte {
	h := hmac.New(sha256.New, hashSecret)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func deriveSigningKey(secret string, t time.Time, region string) []byte {
	// we successively apply the hmac secret in multiple iterations rather then simply
	// write it once (as per the Amazon Keyspaces protocol)
	s := "AWS4" + secret
	h := applyHmac(toCredDateStamp(t), []byte(s))
	h = applyHmac(region, h)
	h = applyHmac("cassandra", h)
	h = applyHmac("aws4_request", h)
	return h
}

func createSignature(canonicalRequest string, t time.Time, signingScope string, signingKey []byte) []byte {
	digest := sha256.Sum256([]byte(canonicalRequest))
	s := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", t.Format("2006-01-02T15:04:05.000Z"), signingScope, hex.EncodeToString(digest[:]))
	return applyHmac(s, signingKey)
}

// buildAWSSignedResponse creates response that can be sent for a SigV4
// challenge this includes both the signature and the metadata supporting
// signature.
func buildSignedResponse(region string, nonce string, accessKeyId string, secret string, sessionToken string, t time.Time) string {
	scope := computeScope(t, region)
	canonicalRequest := formCanonicalRequest(accessKeyId, scope, t, nonce)
	signingKey := deriveSigningKey(secret, t, region)
	signature := createSignature(canonicalRequest, t, scope, signingKey)
	result := fmt.Sprintf("signature=%s,access_key=%s,amzdate=%s", hex.EncodeToString(signature), accessKeyId, t.Format("2006-01-02T15:04:05.000Z"))
	if sessionToken != "" {
		result += fmt.Sprintf(",session_token=%s", sessionToken)
	}
	return result
}
