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
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/srv/app/common"
	libawsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/trace"
)

func (s *signerHandler) serveGitCodeCommitRequest(sessCtx *common.SessionContext, w http.ResponseWriter, req *http.Request) error {
	s.Log.Infof("== code commit req URL %v", req.URL.String())
	s.Log.Infof("== code commit original URL %v", req.Header.Get(common.TeleportOriginalGitURL))

	originalURL, err := url.Parse(req.Header.Get(common.TeleportOriginalGitURL))
	if err != nil {
		return trace.Wrap(err)
	}

	region, err := getRegionFromCodeCommitEndpoint(originalURL.Host)
	if err != nil {
		return trace.Wrap(err)
	}

	credValue, err := s.getSessionCredentialValue(req.Context(), sessCtx, region)
	if err != nil {
		return trace.Wrap(err)
	}

	forwardReq, err := s.makeCodeCommitRequest(req, originalURL, credValue, region)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO figure out why reverse proxy does not work
	//recorder := httplib.NewResponseStatusRecorder(w)
	//s.fwd.ServeHTTP(recorder, forwardReq)

	response, err := http.DefaultClient.Do(forwardReq)
	if err != nil {
		return trace.Wrap(err)
	}
	defer response.Body.Close()

	// TODO audit log
	s.Log.Infof("== code commit response %v %v", response, err)

	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Set(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	n, err := io.Copy(w, response.Body)
	s.Log.Infof("== io.Copy %v %v", n, err)
	return nil
}

func (s *signerHandler) getSessionCredentialValue(ctx context.Context, sessCtx *common.SessionContext, region string) (credentials.Value, error) {
	awsSession, err := s.SessionProvider(ctx, region, sessCtx.App.GetIntegration())
	if err != nil {
		return credentials.Value{}, trace.Wrap(err)
	}
	credentials, err := s.CredentialsGetter.Get(ctx, libawsutils.GetCredentialsRequest{
		Provider:    awsSession,
		Expiry:      sessCtx.Identity.Expires,
		SessionName: sessCtx.Identity.Username,
		RoleARN:     sessCtx.Identity.RouteToApp.AWSRoleARN,
	})
	credValue, err := credentials.GetWithContext(ctx)
	return credValue, trace.Wrap(err)
}

func (s *signerHandler) makeCodeCommitRequest(req *http.Request, originalURL *url.URL, credValue credentials.Value, region string) (*http.Request, error) {
	signTime := s.Clock.Now()
	signature, err := makeCodeCommitSignature(signTime, originalURL, region, credValue)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	forwardReq, err := cloneRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	forwardReq.RequestURI = ""
	forwardReq.URL.Scheme = "https"
	forwardReq.Host = originalURL.Host
	forwardReq.URL.Host = originalURL.Host
	forwardReq.URL.User = url.UserPassword(
		makeCodeCommitUser(credValue),
		signTime.Format(libawsutils.AmzDateTimeFormat)+signature,
	)
	// TODO maybe clean up some headers before sending?
	return forwardReq, nil
}

func makeCodeCommitUser(credValue credentials.Value) string {
	if credValue.SessionToken == "" {
		return credValue.AccessKeyID
	}
	return credValue.AccessKeyID + "%" + credValue.SessionToken
}

const (
	awsShortDateFormat    = "20060102"
	codeCommitServiceName = "codecommit"
)

func makeCodeCommitSignature(signTime time.Time, originalURL *url.URL, region string, credValue credentials.Value) (string, error) {
	signer := &codeCommitSigV4Signer{
		hostname:  originalURL.Hostname(),
		path:      originalURL.Path,
		region:    region,
		signTime:  signTime,
		credValue: credValue,
	}
	signature, err := signer.signature()
	return signature, trace.Wrap(err)
}

// TODO move to lib/utils/aws?
// https://github.com/aws/git-remote-codecommit/blob/master/git_remote_codecommit/__init__.py
type codeCommitSigV4Signer struct {
	hostname  string
	path      string
	region    string
	signTime  time.Time
	credValue credentials.Value
}

func (s *codeCommitSigV4Signer) canonicalRequest() string {
	return fmt.Sprintf("GIT\n%s\n\nhost:%s\n\nhost\n", s.path, s.hostname)
}

func (s *codeCommitSigV4Signer) credentialScope() string {
	parts := []string{
		s.signTime.Format(awsShortDateFormat),
		s.region,
		codeCommitServiceName,
		"aws4_request",
	}
	return strings.Join(parts, "/")
}

func (s *codeCommitSigV4Signer) stringToSign() (string, error) {
	canonicalRequestHash := sha256.New()
	if _, err := canonicalRequestHash.Write([]byte(s.canonicalRequest())); err != nil {
		return "", trace.Wrap(err)
	}
	parts := []string{
		"AWS4-HMAC-SHA256",
		s.signTime.Format("20060102T150405"), // no "Z" suffix
		s.credentialScope(),
		hex.EncodeToString(canonicalRequestHash.Sum(nil)),
	}
	return strings.Join(parts, "\n"), nil
}

func (s *codeCommitSigV4Signer) hmac256(key []byte, data []byte) ([]byte, error) {
	// this function is copyed from internal/v4/hmac.go
	hash := hmac.New(sha256.New, key)
	if _, err := hash.Write(data); err != nil {
		return nil, trace.Wrap(err)
	}
	return hash.Sum(nil), nil
}

func (s *codeCommitSigV4Signer) signature() (string, error) {
	stringToSign, err := s.stringToSign()
	if err != nil {
		return "", trace.Wrap(err)
	}

	rollingParts := []string{
		"AWS4" + s.credValue.SecretAccessKey,
		s.signTime.Format(awsShortDateFormat),
		s.region,
		codeCommitServiceName,
		"aws4_request",
		stringToSign,
	}

	rolling := []byte(rollingParts[0])
	for _, v := range rollingParts[1:] {
		rolling, err = s.hmac256(rolling, []byte(v))
		if err != nil {
			return "", trace.Wrap(err)
		}
	}
	return hex.EncodeToString(rolling), nil
}

// TODO move it to api/utils/aws?
// git-codecommit.ca-central-1.amazonaws.com
func getRegionFromCodeCommitEndpoint(endpoint string) (string, error) {
	if !apiawsutils.IsAWSEndpoint(endpoint) {
		return "", trace.BadParameter("invalid AWS CodeCommit endpoint %v", endpoint)
	}
	if !strings.HasPrefix(endpoint, "git-codecommit.") {
		return "", trace.BadParameter("invalid AWS CodeCommit endpoint %v", endpoint)
	}
	region, _, ok := strings.Cut(strings.TrimPrefix(endpoint, "git-codecommit."), ".")
	if !ok {
		return "", trace.BadParameter("invalid AWS CodeCommit endpoint %v", endpoint)
	}
	if err := apiawsutils.IsValidRegion(region); err != nil {
		return "", trace.Wrap(err)
	}
	return region, nil
}
