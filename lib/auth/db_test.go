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

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_getSnowflakeJWTParams(t *testing.T) {
	type args struct {
		accountName string
		userName    string
		publicKey   []byte
	}
	tests := []struct {
		name        string
		args        args
		wantSubject string
		wantIssuer  string
	}{
		{
			name: "only account locator",
			args: args{
				accountName: "abc123",
				userName:    "user1",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "ABC123.USER1",
			wantIssuer:  "ABC123.USER1.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
		{
			name: "GCP",
			args: args{
				accountName: "abc321.us-central1.gcp",
				userName:    "user1",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "ABC321.USER1",
			wantIssuer:  "ABC321.USER1.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
		{
			name: "AWS",
			args: args{
				accountName: "abc321.us-west-2.aws",
				userName:    "user2",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "ABC321.USER2",
			wantIssuer:  "ABC321.USER2.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
		{
			name: "global",
			args: args{
				accountName: "testaccount-user.global",
				userName:    "user2",
				publicKey:   []byte("fakeKey"),
			},
			wantSubject: "TESTACCOUNT.USER2",
			wantIssuer:  "TESTACCOUNT.USER2.SHA256:q3OCFrBX3MOuBefrAI0e2UgNh5yLGIiSSIuncvcMdGA=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subject, issuer := getSnowflakeJWTParams(tt.args.accountName, tt.args.userName, tt.args.publicKey)

			require.Equal(t, tt.wantSubject, subject)
			require.Equal(t, tt.wantIssuer, issuer)
		})
	}
}
