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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_extractAccountName(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantAccName string
		wantHost    string
		wantErr     bool
	}{
		{
			name:        "correct AWS address - AWS US East (Ohio)",
			uri:         "https://abc123.us-east-2.aws.snowflakecomputing.com",
			wantAccName: "abc123.us-east-2.aws",
			wantHost:    "abc123.us-east-2.aws.snowflakecomputing.com",
		},
		{
			name:        "correct AWS address - AWS US East (Ohio) missing protocol",
			uri:         "abc123.us-east-2.aws.snowflakecomputing.com",
			wantAccName: "abc123.us-east-2.aws",
			wantHost:    "abc123.us-east-2.aws.snowflakecomputing.com",
		},
		{
			name:        "correct AWS address - AWS US West (Oregon)",
			uri:         "abc123.snowflakecomputing.com",
			wantAccName: "abc123",
			wantHost:    "abc123.snowflakecomputing.com",
		},
		{
			name:        "correct AWS address - AWS EU (Frankfurt)",
			uri:         "abc123.eu-central-1.snowflakecomputing.com",
			wantAccName: "abc123.eu-central-1",
			wantHost:    "abc123.eu-central-1.snowflakecomputing.com",
		},
		{
			name:        "correct GCP address",
			uri:         "abc123.us-central1.gcp.snowflakecomputing.com",
			wantAccName: "abc123.us-central1.gcp",
			wantHost:    "abc123.us-central1.gcp.snowflakecomputing.com",
		},
		{
			name:        "correct Azure address",
			uri:         "abc123.central-us.azure.snowflakecomputing.com",
			wantAccName: "abc123.central-us.azure",
			wantHost:    "abc123.central-us.azure.snowflakecomputing.com",
		},
		{
			name:        "user account query is provided",
			uri:         "abc123.us-east-2.aws.snowflakecomputing.com?account=someAccount",
			wantAccName: "someAccount",
			wantHost:    "abc123.us-east-2.aws.snowflakecomputing.com",
		},
		{
			name:    "empty returns error",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "incorrect url returns error",
			uri:     "blah",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAccName, gotHost, err := parseConnectionString(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseConnectionString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, tt.wantAccName, gotAccName)
			require.Equal(t, tt.wantHost, gotHost)
		})
	}
}

func Test_extractSnowflakeToken(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		want    string
	}{
		{
			name: "extract correct header",
			headers: map[string][]string{
				"Authorization": {"Snowflake Token=\"token123\""},
			},
			want: "token123",
		},
		{
			name: "empty Authorization returns nothing",
			headers: map[string][]string{
				"Authorization": {},
			},
			want: "",
		},
		{
			name:    "missing Authorization returns nothing",
			headers: map[string][]string{},
			want:    "",
		},
		{
			name: "incorrect format returns nothing",
			headers: map[string][]string{
				"Authorization": {"Token=\"token123\""},
			},
			want: "",
		},
		{
			name: "incorrect format returns nothing #2",
			headers: map[string][]string{
				"Authorization": {"Snowflake Token=\""},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSnowflakeToken(tt.headers)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_replaceLoginReqToken(t *testing.T) {
	const loginResponse = `{"data":{"CLIENT_APP_ID":"","CLIENT_APP_VERSION":"","SVN_REVISION":"","ACCOUNT_NAME":"testAccountName","AUTHENTICATOR":"SNOWFLAKE_JWT","CLIENT_ENVIRONMENT":null,"LOGIN_NAME":"alice","TOKEN":"testJWT"}}`

	type args struct {
		loginReq    map[string]interface{}
		jwtToken    string
		accountName string
		loginName   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "base case",
			args: args{
				loginReq: map[string]interface{}{
					"TOKEN":        "testJWT",
					"ACCOUNT_NAME": "testAccountName",
				},
				jwtToken:    "testJWT",
				loginName:   "alice",
				accountName: "testAccountName",
			},
			want: loginResponse,
		},
		{
			name: "remove password",
			args: args{
				loginReq: map[string]interface{}{
					"TOKEN":        "testJWT",
					"ACCOUNT_NAME": "testAccountName",
					"PASSWORD":     "password",
				},
				jwtToken:    "testJWT",
				loginName:   "alice",
				accountName: "testAccountName",
			},
			want: loginResponse,
		},
		{
			name: "remove username",
			args: args{
				loginReq: map[string]interface{}{
					"TOKEN":        "testJWT",
					"ACCOUNT_NAME": "testAccountName",
					"LOGIN_NAME":   "alice",
				},
				jwtToken:    "testJWT",
				loginName:   "alice",
				accountName: "testAccountName",
			},
			want: loginResponse,
		},
		{
			name: "replace authenticator username",
			args: args{
				loginReq: map[string]interface{}{
					"TOKEN":         "testJWT",
					"ACCOUNT_NAME":  "testAccountName",
					"AUTHENTICATOR": "PASSWORD",
				},
				jwtToken:    "testJWT",
				loginName:   "alice",
				accountName: "testAccountName",
			},
			want: loginResponse,
		},
		{
			name: "replace login name",
			args: args{
				loginReq: map[string]interface{}{
					"TOKEN":         "testJWT",
					"ACCOUNT_NAME":  "testAccountName",
					"AUTHENTICATOR": "PASSWORD",
					"LOGIN_NAME":    "bob",
				},
				jwtToken:    "testJWT",
				loginName:   "alice",
				accountName: "testAccountName",
			},
			want: loginResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := json.Marshal(tt.args.loginReq)
			require.NoError(t, err)

			got, err := replaceLoginReqToken(payload, tt.args.jwtToken, tt.args.accountName, tt.args.loginName)

			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(got))
		})
	}
}

func TestEngine_processLoginResponse(t *testing.T) {
	type args struct {
		bodyBytes       []byte
		createSessionFn func(tokens sessionTokens) (string, string, error)
	}
	tests := []struct {
		name        string
		args        args
		want        string
		assertCache func(t *testing.T, cache *tokenCache)
		wantErr     bool
	}{
		{
			name: "success",
			args: args{
				bodyBytes: []byte(testLoginResponse),
				createSessionFn: func(tokens sessionTokens) (string, string, error) {
					return "token1", "token2", nil
				},
			},
			want: `{"code":null, "data":{"masterToken":"Teleport:token2", "masterValidityInSeconds":14400, "token":"Teleport:token1", "validityInSeconds":3600}, "message":null, "success":true}`,
			assertCache: func(t *testing.T, cache *tokenCache) {
				require.Equal(t, "test-token-123", cache.getToken("token1"))
				require.Equal(t, "master-token-123", cache.getToken("token2"))
			},
		},
		{
			name: "additional fields are not removed",
			args: args{
				bodyBytes: []byte(testLoginRespExtraField),
				createSessionFn: func(tokens sessionTokens) (string, string, error) {
					return "token-session-345", "token-master-567", nil
				},
			},
			want: `{"code":null, "data":{"masterToken":"Teleport:token-master-567", "masterValidityInSeconds":14400, "token":"Teleport:token-session-345", "validityInSeconds":3600, "additionalField": 123}, "message":null, "success":true}`,
			assertCache: func(t *testing.T, cache *tokenCache) {
				require.Equal(t, "test-token-123", cache.getToken("token-session-345"))
				require.Equal(t, "master-token-123", cache.getToken("token-master-567"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{
				tokens: newTokenCache(),
			}
			got, err := e.processLoginResponse(tt.args.bodyBytes, tt.args.createSessionFn)
			if (err != nil) != tt.wantErr {
				t.Errorf("processLoginResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.JSONEq(t, tt.want, string(got))
			tt.assertCache(t, &e.tokens)
		})
	}
}

const testLoginRespExtraField = `
{
  "data": {
    "token": "test-token-123",
	"validityInSeconds": 3600,
    "masterToken": "master-token-123",
	"masterValidityInSeconds": 14400,
	"additionalField": 123
  },
  "success": true
}`
