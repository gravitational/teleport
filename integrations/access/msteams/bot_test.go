// Copyright 2024 Gravitational, Inc
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

package msteams

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func mustParseURL(t *testing.T, urlString string) *url.URL {
	parsedURL, err := url.Parse(urlString)
	require.NoError(t, err)
	return parsedURL
}

func Test_CheckChannelURL(t *testing.T) {
	b := &Bot{
		log: utils.NewSlogLoggerForTests(),
	}
	tests := []struct {
		name             string
		url              string
		expectedUserData *Channel
		validURL         bool
	}{
		{
			name: "Valid URL",
			url:  "https://teams.microsoft.com/l/channel/19%3ae06a7383ed98468f90217a35fa1980d7%40thread.tacv2/Approval%2520Channel%25202?groupId=f2b3c8ed-5502-4449-b76f-dc3acea81f1c&tenantId=ff882432-09b0-437b-bd22-ca13c0037ded",
			expectedUserData: &Channel{
				Name:   "Approval%20Channel%202",
				Group:  "f2b3c8ed-5502-4449-b76f-dc3acea81f1c",
				Tenant: "ff882432-09b0-437b-bd22-ca13c0037ded",
				URL:    *mustParseURL(t, "https://teams.microsoft.com/l/channel/19%3ae06a7383ed98468f90217a35fa1980d7%40thread.tacv2/Approval%2520Channel%25202?groupId=f2b3c8ed-5502-4449-b76f-dc3acea81f1c&tenantId=ff882432-09b0-437b-bd22-ca13c0037ded"),
				ChatID: "19:e06a7383ed98468f90217a35fa1980d7@thread.tacv2",
			},
			validURL: true,
		},
		{
			name:             "Invalid URL (no tenant)",
			url:              "https://teams.microsoft.com/l/channel/19%3ae06a7383ed98468f90217a35fa1980d7%40thread.tacv2/Approval%2520Channel%25202?groupId=f2b3c8ed-5502-4449-b76f-dc3acea81f1c",
			expectedUserData: nil,
			validURL:         false,
		},
		{
			name:             "Invalid URL (wrong length)",
			url:              "https://teams.microsoft.com/channel/19%3ae06a7383ed98468f90217a35fa1980d7%40thread.tacv2/Approval%2520Channel%25202?groupId=f2b3c8ed-5502-4449-b76f-dc3acea81f1c&tenantId=ff882432-09b0-437b-bd22-ca13c0037ded",
			expectedUserData: nil,
			validURL:         false,
		},
		{
			name:             "Email",
			url:              "foo@example.com",
			expectedUserData: nil,
			validURL:         false,
		},
		{
			name:             "Not an URL",
			url:              "This is not an url 🙂",
			expectedUserData: nil,
			validURL:         false,
		},
		{
			name: "troubleshoot",
			url:  "https://teams.microsoft.com/l/channel/19%3A0cb90b248ba740a9adb4c732f227f3bc%40thread.tacv2/Teleport-MsTeams-Nara?groupId=4030a14f-55c5-4da5-9d98-b318f7c41481&tenantId=ff882432-09b0-437b-bd22-ca13c0037ded",
			expectedUserData: &Channel{
				Name:   "Teleport-MsTeams-Nara",
				Group:  "4030a14f-55c5-4da5-9d98-b318f7c41481",
				Tenant: "ff882432-09b0-437b-bd22-ca13c0037ded",
				URL:    *mustParseURL(t, "https://teams.microsoft.com/l/channel/19%3A0cb90b248ba740a9adb4c732f227f3bc%40thread.tacv2/Teleport-MsTeams-Nara?groupId=4030a14f-55c5-4da5-9d98-b318f7c41481&tenantId=ff882432-09b0-437b-bd22-ca13c0037ded"),
				ChatID: "19:0cb90b248ba740a9adb4c732f227f3bc@thread.tacv2",
			},
			validURL: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, ok := b.checkChannelURL(tc.url)
			require.Equal(t, tc.validURL, ok)
			if tc.validURL {
				require.Equal(t, tc.expectedUserData, data)
			}
		})
	}
}
