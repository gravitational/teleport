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
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "correct AWS address",
			uri:  "abc123.us-east-2.aws.snowflakecomputing.com",
			want: "abc123.us-east-2.aws",
		},
		{
			name: "correct AWS address",
			uri:  "abc123.snowflakecomputing.com",
			want: "abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAccountName(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractAccountName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, tt.want, got)
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
	const loginResponse = `{"data":{"CLIENT_APP_ID":"","CLIENT_APP_VERSION":"","SVN_REVISION":"","ACCOUNT_NAME":"testAccountName","AUTHENTICATOR":"SNOWFLAKE_JWT","CLIENT_ENVIRONMENT":null,"TOKEN":"testJWT"}}`

	type args struct {
		loginReq    map[string]interface{}
		jwtToken    string
		accountName string
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
				accountName: "testAccountName",
			},
			want: loginResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := json.Marshal(tt.args.loginReq)
			require.NoError(t, err)

			got, err := replaceLoginReqToken(payload, tt.args.jwtToken, tt.args.accountName)

			require.NoError(t, err)
			require.JSONEq(t, tt.want, string(got))
		})
	}
}
