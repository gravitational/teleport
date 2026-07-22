/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package snowflake

import (
	"encoding/json"
	"fmt"
	"reflect"
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

type loginResponseData struct {
	Token                   string `json:"token"`
	MasterToken             string `json:"masterToken"`
	ValidityInSeconds       int64  `json:"validityInSeconds"`
	MasterValidityInSeconds int64  `json:"masterValidityInSeconds"`

	// allFields contains all fields from the JSON. Those fields will
	// be added when marshaling JSON.
	allFields map[string]any
}

func (l *loginResponseData) MarshalJSON() ([]byte, error) {
	elems := reflect.TypeOf(l).Elem()

	for i := range elems.NumField() {
		jsonTag, ok := elems.Field(i).Tag.Lookup("json")
		if !ok {
			continue
		}
		l.allFields[jsonTag] = reflect.ValueOf(l).Elem().Field(i).Interface()
	}

	return json.Marshal(l.allFields)
}

func (l *loginResponseData) UnmarshalJSON(data []byte) error {
	type _loginResponseData loginResponseData
	var respData _loginResponseData

	if err := json.Unmarshal(data, &respData); err != nil {
		return trace.Wrap(err)
	}

	*l = loginResponseData(respData)

	if err := json.Unmarshal(data, &l.allFields); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// loginResponse is the payload returned by the /queries/v1/query-request endpoint.
type loginResponse struct {
	Data    loginResponseData `json:"data"`
	Code    any               `json:"code"`
	Message any               `json:"message"`
	Success bool              `json:"success"`
}

func (l *loginResponse) checkAndGetTokens() (sessionTokens, error) {
	if l.Data.Token == "" {
		return sessionTokens{}, trace.Errorf("token field in login response is not set")
	}

	if l.Data.MasterToken == "" {
		return sessionTokens{}, trace.Errorf("masterToken field in login response is not set")
	}

	if l.Data.ValidityInSeconds == 0 {
		return sessionTokens{}, trace.Errorf("validityInSeconds field in login response is not set")
	}

	if l.Data.MasterValidityInSeconds == 0 {
		return sessionTokens{}, trace.Errorf("masterValidityInSeconds field in login response is not set")
	}

	return sessionTokens{
		tokenTTL{
			token: l.Data.Token,
			ttl:   time.Duration(l.Data.ValidityInSeconds),
		},
		tokenTTL{
			token: l.Data.MasterToken,
			ttl:   time.Duration(l.Data.MasterValidityInSeconds),
		}}, nil
}

// renewSessionRequest is the payload sent to the /session/token-request endpoint.
type renewSessionRequest struct {
	OldSessionToken string `json:"oldSessionToken"`
	RequestType     string `json:"requestType"`
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
	Parameters map[string]any               `json:"parameters,omitempty"`
	Bindings   map[string]execBindParameter `json:"bindings,omitempty"`
	BindStage  string                       `json:"bindStage,omitempty"`
}

type execBindParameter struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
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

func queryParametersToSlice(parameters map[string]any) []string {
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
