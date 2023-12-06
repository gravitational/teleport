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
