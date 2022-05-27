/*

 Copyright 2022 Gravitational, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.


*/

package snowflake

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/trace"
)

// loginRequest is the payload sent to /queries/v1/query-request endpoint.
type loginRequest struct {
	Data struct {
		ClientAppID             string          `json:"CLIENT_APP_ID"`
		ClientAppVersion        string          `json:"CLIENT_APP_VERSION"`
		SvnRevision             string          `json:"SVN_REVISION"`
		AccountName             string          `json:"ACCOUNT_NAME"`
		LoginName               string          `json:"LOGIN_NAME,omitempty"`
		Password                string          `json:"PASSWORD,omitempty"`
		RawSAMLResponse         string          `json:"RAW_SAML_RESPONSE,omitempty"`
		ExtAuthnDuoMethod       string          `json:"EXT_AUTHN_DUO_METHOD,omitempty"`
		Passcode                string          `json:"PASSCODE,omitempty"`
		Authenticator           string          `json:"AUTHENTICATOR,omitempty"`
		SessionParameters       json.RawMessage `json:"SESSION_PARAMETERS,omitempty"`
		ClientEnvironment       json.RawMessage `json:"CLIENT_ENVIRONMENT"`
		BrowserModeRedirectPort string          `json:"BROWSER_MODE_REDIRECT_PORT,omitempty"`
		ProofKey                string          `json:"PROOF_KEY,omitempty"`
		Token                   string          `json:"TOKEN,omitempty"`
	} `json:"data"`
}

// loginResponse is the payload returned by the /queries/v1/query-request endpoint.
type loginResponse struct {
	// use map here to not remove any fields when marshaling back.
	Data map[string]interface{} `json:"data"`

	Code    interface{} `json:"code"`
	Message interface{} `json:"message"`
	Success bool        `json:"success"`
}

// decodeLoginResponse decodes the bodyBytes as loginResponse struct. It uses json.Number to decode number.
func decodeLoginResponse(bodyBytes []byte) (*loginResponse, error) {
	loginResp := &loginResponse{}
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	// Use numbers to not modify the numbers values when processing payload.
	decoder.UseNumber()
	if err := decoder.Decode(loginResp); err != nil {
		return nil, trace.Wrap(err)
	}
	return loginResp, nil
}

func (l *loginResponse) getTokens() (sessionTokens, error) {
	getField := func(name string) (string, error) {
		dataToken, found := l.Data[name]
		if !found {
			return "", trace.Errorf("Snowflake login response doesn't contain expected %s field", name)
		}

		token, ok := dataToken.(string)
		if !ok {
			return "", trace.Errorf("%s field returned by Snowflake API expected to be a string, got %T", name, dataToken)
		}

		return token, nil
	}

	getFieldInt := func(name string) (int64, error) {
		dataToken, found := l.Data[name]
		if !found {
			return 0, trace.Errorf("Snowflake login response doesn't contain expected %s field", name)
		}

		validFor, ok := dataToken.(json.Number)
		if !ok {
			return 0, trace.Errorf("%s field returned by Snowflake API expected to be a number, got %T", name, dataToken)
		}

		return validFor.Int64()
	}

	// Use dynamic get function as we modify the request on the fly.
	snowflakeSessionToken, err := getField("token")
	if err != nil {
		return sessionTokens{}, trace.Wrap(err)
	}

	validInSec, err := getFieldInt("validityInSeconds")
	if err != nil {
		return sessionTokens{}, trace.Wrap(err)
	}

	snowflakeMasterToken, err := getField("masterToken")
	if err != nil {
		return sessionTokens{}, trace.Wrap(err)
	}

	masterValidInSec, err := getFieldInt("masterValidityInSeconds")
	if err != nil {
		return sessionTokens{}, trace.Wrap(err)
	}

	return sessionTokens{
		tokenTTL{
			token: snowflakeSessionToken,
			ttl:   time.Duration(validInSec),
		},
		tokenTTL{
			token: snowflakeMasterToken,
			ttl:   time.Duration(masterValidInSec),
		}}, nil
}

// renewSessionRequest is the payload sent to the /session/token-request endpoint.
type renewSessionRequest struct {
	OldSessionToken string `json:"oldSessionToken"`
	RequestType     string `json:"requestType"` // "RENEW"
}

// renewSessionResponse is the payload returned by the /session/token-request endpoint.
type renewSessionResponse struct {
	Data struct {
		SessionToken        string        `json:"sessionToken"`
		ValidityInSecondsST time.Duration `json:"validityInSecondsST"`
		MasterToken         string        `json:"masterToken"`
		ValidityInSecondsMT time.Duration `json:"validityInSecondsMT"`
		SessionID           int64         `json:"sessionId"`
	} `json:"data"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Success bool   `json:"success"`
}

// queryRequest is the request body sent to /queries/v1/query-request endpoint.
// In our case we only care about SQLText as this is the field that contain the
// SQL query that we need to log.
type queryRequest struct {
	SQLText    string                       `json:"sqlText"`
	Parameters map[string]interface{}       `json:"parameters,omitempty"`
	Bindings   map[string]execBindParameter `json:"bindings,omitempty"`
	BindStage  string                       `json:"bindStage,omitempty"`
}

type execBindParameter struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

func (q *queryRequest) paramsToSlice() []string {
	args := make([]string, 0)

	args = append(args, queryBindingsToSlice(q.Bindings)...)
	args = append(args, queryParametersToSlice(q.Parameters)...)

	if q.BindStage != "" {
		args = append(args, fmt.Sprintf("bindStage:%s", q.BindStage))
	}

	return args
}

func queryParametersToSlice(parameters map[string]interface{}) []string {
	params := make([]string, 0)

	for k, v := range parameters {
		params = append(params, fmt.Sprintf("parameters:{%v:%v}", k, v))
	}

	return params
}

func queryBindingsToSlice(bindings map[string]execBindParameter) []string {
	values := make([]string, 0)

	for k, v := range bindings {
		values = append(values, fmt.Sprintf("bindings:{%v:[%v,%v]}", k, v.Type, v.Value))
	}

	return values
}
