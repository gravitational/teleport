// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alpnproxy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
)

type AzureMSIMiddleware struct {
	Identity string
	TenantID string
	ClientID string

	// Clock is used to override time in tests.
	Clock clockwork.Clock
	// Log is the Logger.
	Log logrus.FieldLogger
}

var _ LocalProxyHTTPMiddleware = &AzureMSIMiddleware{}

func (m *AzureMSIMiddleware) OnStart(ctx context.Context, lp *LocalProxy) error {
	if m.Log == nil {
		m.Log = lp.cfg.Log
	}

	if m.Clock == nil {
		m.Clock = lp.cfg.Clock
	}

	if m.Identity == "" {
		return trace.BadParameter("missing Identity")
	}
	if m.TenantID == "" {
		return trace.BadParameter("missing TenantID")
	}
	if m.ClientID == "" {
		return trace.BadParameter("missing ClientID")
	}

	return nil
}

func (m *AzureMSIMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	if req.Host == types.TeleportAzureMSIEndpoint {
		err := m.msiEndpoint(rw, req)
		if err != nil {
			m.Log.Warnf("Bad MSI request: %v", err)
			trace.WriteError(rw, trace.Wrap(err))
		}
		return true
	}

	return false
}

func (m *AzureMSIMiddleware) msiEndpoint(rw http.ResponseWriter, req *http.Request) error {
	// request validation
	metadata := req.Header.Get("Metadata")
	if metadata != "true" {
		return trace.BadParameter("expected Metadata header with value 'true'")
	}

	err := req.ParseForm()
	if err != nil {
		return trace.Wrap(err)
	}

	resource := req.Form.Get("resource")
	if resource == "" {
		return trace.BadParameter("missing value for parameter 'resource'")
	}

	// check that msi_res_id matches expected Azure Identity
	requestedAzureIdentity := req.Form.Get("msi_res_id")
	if requestedAzureIdentity != m.Identity {
		m.Log.Warnf("Requested unexpected identity %q, expected %q", requestedAzureIdentity, m.Identity)
		return trace.BadParameter("unexpected value for parameter 'msi_res_id': %v", requestedAzureIdentity)
	}

	respBody, err := m.fetchMSILoginResp(resource)
	if err != nil {
		return trace.Wrap(err)
	}

	m.Log.Infof("MSI: returning token for identity %v", m.Identity)

	rw.Header().Add("Content-Type", "application/json; charset=utf-8")
	rw.Header().Add("Content-Length", fmt.Sprintf("%v", len(respBody)))
	rw.WriteHeader(200)
	_, _ = rw.Write(respBody)
	return nil
}

func (m *AzureMSIMiddleware) fetchMSILoginResp(resource string) ([]byte, error) {
	now := m.Clock.Now()

	notBefore := now.Add(-time.Minute)
	expiresOn := now.Add(time.Hour * 24 * 365)
	expiresIn := int64(now.Sub(expiresOn).Seconds())

	accessToken := map[string]any{
		"iat":           notBefore.Unix(),
		"tid":           m.TenantID,
		"teleport_mark": uuid.New().String(),
		"resource":      resource,
	}

	accessTokenJwt, err := m.toJWT(accessToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := map[string]any{
		"access_token":   accessTokenJwt,
		"client_id":      m.ClientID,
		"not_before":     notBefore.Unix(),
		"expires_on":     expiresOn.Unix(),
		"expires_in":     expiresIn,
		"ext_expires_in": expiresIn,
		"token_type":     "Bearer",
		"resource":       resource,
	}

	out, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// toJWT TODO: although this works in practice, it is also terribly hackish and in need of proper implementation.
func (m *AzureMSIMiddleware) toJWT(token map[string]any) (any, error) {
	bs, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	claims := base64.StdEncoding.EncodeToString(bs)

	jwt := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9." + claims + ".NKYW_avp9VriiaaYwL3AGj2JLoZRnlvfJETrGWZJT3JpQi8XC9HwpszxrnQLB689W3e8a481aH6b5C4bWucXlgO5wJ5g28mqEpdVwMypRMoICQrLUo7stPNX6iiWZdjn4YkurFw0FWbOjy-B-t05SiVCB4VikX5uuqA1CqZPzmfKibW1hmhYlsXQIRtz7HKDj7pU3Eu16ggtwOtWeVi9XQiMQ0CA3UfWw80VE_qiQvkVQsPY6dwX9M-7xHgieB7LqdVRy7sr-Ok_UX8oy4nydS-8lKHRBeKp8_EcvCZ3cyY6kcdMEEIuwVDuL2f3oJ3arUwjvzLcudQE9cPBdqdX0g"

	return jwt, nil
}
