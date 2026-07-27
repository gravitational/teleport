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

package createsession

import (
	"bytes"
	"cmp"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// CreateSessionRequest is a request to create a session with AWS IAM Roles Anywhere.
type CreateSessionRequest struct {
	// TrustAnchorARN is the ARN of the AWS IAM Roles Anywhere Trust Anchor.
	TrustAnchorARN string
	// ProfileARN is the ARN of the AWS IAM Roles Anywhere Profile.
	ProfileARN string
	// RoleARN is the ARN of the AWS IAM Role to generate credentials.
	RoleARN string
	// RoleSessionName is the name of the session to create.
	// It will be visible in AWS CloudTrail logs.
	// Only set this value if the Profile Accepts Custom Session Names.
	RoleSessionName string
	// Duration is the duration of the session.
	// Omitting this value means the session will be valid for the profile's default duration.
	// Valid values are between 15 minutes and 12 hours.
	DurationSeconds *int

	// Certificate is the certificate that will be exchanged to obtain the credentials.
	Certificate *x509.Certificate
	// IntermediateCertificates are the intermediate certificates needed to
	// chain Certificate to a trusted root.
	IntermediateCertificates []*x509.Certificate
	// PrivateKey is the private key that will be used to sign the request.
	PrivateKey crypto.Signer
	// RegionOverride is an optional AWS region override.
	// If not provided, this is inferred from the TrustAnchorARN.
	RegionOverride string

	awsRegion string

	// HTTPClient is the HTTP client used to make the request.
	// If not set, a default HTTP client will be used.
	// Used for testing purposes.
	HTTPClient utils.HTTPDoClient

	// clock is the clock used to get the current time.
	// If not set, a real clock will be used.
	// Used for testing purposes.
	clock clockwork.Clock
}

func (req *CreateSessionRequest) checkAndSetDefaults() error {
	raTrustAnchor, err := arn.Parse(req.TrustAnchorARN)
	if err != nil {
		return trace.BadParameter("invalid roles anywhere trust anchor arn: %v", err)
	}

	_, err = arn.Parse(req.ProfileARN)
	if err != nil {
		return trace.BadParameter("invalid roles anywhere profile arn: %v", err)
	}

	_, err = arn.Parse(req.RoleARN)
	if err != nil {
		return trace.BadParameter("invalid iam role arn: %v", err)
	}

	if req.DurationSeconds != nil {
		const minDurationSecs = 15 * 60      // 15 minutes
		const maxDurationSecs = 12 * 60 * 60 // 12 hours
		if *req.DurationSeconds < minDurationSecs || *req.DurationSeconds > maxDurationSecs {
			return trace.BadParameter("duration must be between 15 minutes and 12 hours")
		}
	}

	if req.Certificate == nil {
		return trace.BadParameter("certificate is required")
	}

	if req.PrivateKey == nil {
		return trace.BadParameter("private key is required")
	}

	if req.HTTPClient == nil {
		httpClient, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		req.HTTPClient = httpClient
	}

	req.awsRegion = cmp.Or(req.RegionOverride, raTrustAnchor.Region)
	if err := aws.IsValidRegion(req.awsRegion); err != nil {
		return trace.BadParameter("invalid region: %v", err)
	}

	req.clock = cmp.Or(req.clock, clockwork.NewRealClock())

	return nil
}

// From https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html
// > Algorithm. As described above, instead of AWS4-HMAC-SHA256, the algorithm
// > field will have the values of the form AWS4-X509-RSA-SHA256 or
// > AWS4-X509-ECDSA-SHA256, depending on whether an RSA or Elliptic Curve
// > algorithm is used. This, in turn, is determined by the key bound to the
// > signing certificate.
const (
	algoRSA   = "AWS4-X509-RSA-SHA256"
	algoECDSA = "AWS4-X509-ECDSA-SHA256"
)

func algoForKey(key crypto.Signer) (string, error) {
	switch key.(type) {
	case *rsa.PrivateKey:
		return algoRSA, nil
	case *ecdsa.PrivateKey:
		return algoECDSA, nil
	default:
		return "", trace.BadParameter("unsupported key type: %T", key)
	}
}

func signWithKey(key crypto.Signer, hash []byte) ([]byte, error) {
	switch key := key.(type) {
	case *ecdsa.PrivateKey:
		signature, err := ecdsa.SignASN1(rand.Reader, key, hash)
		if err != nil {
			return nil, trace.Wrap(err, "signing with ECDSA")
		}
		return signature, nil
	case *rsa.PrivateKey:
		signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash)
		if err != nil {
			return nil, fmt.Errorf("signing with RSA: %w", err)
		}
		return signature, nil
	default:
		return nil, trace.BadParameter("unsupported key type: %T", key)
	}
}

// CreateSession exchanges a certificate for AWS credentials using the AWS IAM Roles Anywhere service.
// This method is based on the following guide:
// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html
func CreateSession(ctx context.Context, req CreateSessionRequest) (*CreateSessionResponse, error) {
	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Task 1: Create a canonical request
	// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html#authentication-task1

	createSessionRequestBody := struct {
		ProfileARN      string `json:"profileArn"`
		RoleARN         string `json:"roleArn"`
		TrustAnchorARN  string `json:"trustAnchorArn"`
		RoleSessionName string `json:"roleSessionName,omitempty"`
		DurationSeconds *int   `json:"durationSeconds,omitempty"`
	}{
		ProfileARN:      req.ProfileARN,
		RoleARN:         req.RoleARN,
		TrustAnchorARN:  req.TrustAnchorARN,
		RoleSessionName: req.RoleSessionName,
		DurationSeconds: req.DurationSeconds,
	}

	canonicalRequestBody, err := json.Marshal(createSessionRequestBody)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	canonicalRequestURL := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("rolesanywhere.%v.amazonaws.com", req.awsRegion),
		Path:   "/sessions", // Task 1-2
	}

	canonicalRequest, err := http.NewRequestWithContext(ctx,
		http.MethodPost, // Task 1-1
		canonicalRequestURL.String(),
		bytes.NewReader(canonicalRequestBody),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Task 1-4,5
	formatedDate := req.clock.Now().UTC().Round(time.Second).Format("20060102T150405Z")
	signedHeaders := "content-type;host;x-amz-date;x-amz-x509"
	canonicalRequest.Header.Set("Content-Type", "application/json")
	canonicalRequest.Header.Set("Host", canonicalRequest.Host)
	canonicalRequest.Header.Set("X-Amz-Date", formatedDate)
	canonicalRequest.Header.Set("X-Amz-X509", base64.StdEncoding.EncodeToString(req.Certificate.Raw))
	if len(req.IntermediateCertificates) > 0 {
		encodedDelimitedIntermediates := strings.Builder{}
		for i, cert := range req.IntermediateCertificates {
			encodedDelimitedIntermediates.WriteString(base64.StdEncoding.EncodeToString(cert.Raw))
			if i < len(req.IntermediateCertificates)-1 {
				encodedDelimitedIntermediates.WriteString(",")
			}
		}
		canonicalRequest.Header.Set("X-Amz-X509-Chain", encodedDelimitedIntermediates.String())
		signedHeaders += ";x-amz-x509-chain"
	}

	// Task 1-6
	canonicalRequestBodyHash := sha256.Sum256(canonicalRequestBody)

	// Task 1-7
	canonicalReqStrBuilder := strings.Builder{}
	canonicalReqStrBuilder.WriteString("POST\n")
	canonicalReqStrBuilder.WriteString("/sessions\n")
	// Blank line after method + path
	canonicalReqStrBuilder.WriteString("\n")
	// Headers
	canonicalReqStrBuilder.WriteString("content-type:application/json\n")
	canonicalReqStrBuilder.WriteString("host:" + canonicalRequest.Header.Get("Host") + "\n")
	canonicalReqStrBuilder.WriteString("x-amz-date:" + canonicalRequest.Header.Get("X-Amz-Date") + "\n")
	canonicalReqStrBuilder.WriteString("x-amz-x509:" + canonicalRequest.Header.Get("X-Amz-X509") + "\n")
	if chain := canonicalRequest.Header.Get("X-Amz-X509-Chain"); chain != "" {
		canonicalReqStrBuilder.WriteString("x-amz-x509-chain:" + chain + "\n")
	}
	// Blank line after headers
	canonicalReqStrBuilder.WriteString("\n")
	// List of signed headers
	canonicalReqStrBuilder.WriteString(signedHeaders + "\n")
	// Body hash encoded as hex
	fmt.Fprintf(&canonicalReqStrBuilder, "%x", canonicalRequestBodyHash)

	// Task 1-8
	canonicalRequestStr := canonicalReqStrBuilder.String()
	canonicalRequestHashBytes := sha256.New()
	canonicalRequestHashBytes.Write([]byte(canonicalRequestStr))
	canonicalRequestHash := hex.EncodeToString(canonicalRequestHashBytes.Sum(nil))

	// Task 2: Create a string to sign
	// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html#authentication-task2
	algorithm, err := algoForKey(req.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentialScope := formatedDate[:8] + "/" + req.awsRegion + "/rolesanywhere/aws4_request"

	stringToSign := algorithm + "\n" +
		formatedDate + "\n" +
		credentialScope + "\n" +
		canonicalRequestHash

	// Task 3: Calculate the signature
	// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-sign-process.html#authentication-task3
	signatureHash := sha256.Sum256([]byte(stringToSign))
	signature, err := signWithKey(req.PrivateKey, signatureHash[:])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Task 4-2
	credentialString := req.Certificate.SerialNumber.String() + "/" + credentialScope
	// Task 4-3
	canonicalRequest.Header.Set("Authorization",
		algorithm+" "+
			"Credential="+credentialString+", "+
			"SignedHeaders="+signedHeaders+", "+
			"Signature="+hex.EncodeToString(signature),
	)

	resp, err := req.HTTPClient.Do(canonicalRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer resp.Body.Close()

	respBody, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, trace.ReadError(resp.StatusCode, respBody)
	}

	// https://docs.aws.amazon.com/rolesanywhere/latest/userguide/authentication-create-session.html#response-syntax
	createSessionResp := struct {
		CredentialSet []struct {
			Credentials struct {
				AccessKeyId     string `json:"accessKeyId"`
				SecretAccessKey string `json:"secretAccessKey"`
				SessionToken    string `json:"sessionToken"`
				Expiration      string `json:"expiration"`
			} `json:"credentials"`
		} `json:"credentialSet"`
	}{}

	if err := json.Unmarshal(respBody, &createSessionResp); err != nil {
		return nil, trace.BadParameter("parsing response: %v", err)
	}

	if len(createSessionResp.CredentialSet) == 0 {
		return nil, trace.BadParameter("no credentials received from rolesanywhere.CreateSession API")
	}

	credentials := createSessionResp.CredentialSet[0].Credentials

	return &CreateSessionResponse{
		Version:         1,
		AccessKeyID:     credentials.AccessKeyId,
		SecretAccessKey: credentials.SecretAccessKey,
		SessionToken:    credentials.SessionToken,
		Expiration:      credentials.Expiration,
	}, nil
}

// CreateSessionResponse contains the response from the CreateSession API call.
// It contains the credentials that can be used to access AWS APIs.
type CreateSessionResponse struct {
	// Version is always 1.
	Version int
	// AccessKeyID is the AWS access key ID.
	AccessKeyID string
	// SecretAccessKey is the AWS secret access key.
	SecretAccessKey string
	// SessionToken is the AWS session token.
	SessionToken string
	// Expiration is the expiration time of the credentials, format: RFC3339.
	Expiration string
}
