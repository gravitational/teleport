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

package email

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gopkg.in/mail.v2"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
)

func TestRecipients(t *testing.T) {
	testCases := []struct {
		desc             string
		in               string
		expectErr        require.ErrorAssertionFunc
		expectRecipients common.RawRecipientsMap
	}{
		{
			desc: "test delivery recipients",
			in: `
            [mailgun]
            domain = "x"
            private_key = "y"
            [delivery]
            sender = "email@example.org"
			recipients = ["email1@example.org","email2@example.org"]
			`,
			expectRecipients: common.RawRecipientsMap{
				types.Wildcard: []string{"email1@example.org", "email2@example.org"},
			},
		},
		{
			desc: "test role_to_recipients",
			in: `
            [mailgun]
            domain = "x"
            private_key = "y"
            [delivery]
            sender = "email@example.org"

			[role_to_recipients]
			"dev" = ["dev@example.org","sre@example.org"]
			"*" = "admin@example.org"
			`,
			expectRecipients: common.RawRecipientsMap{
				"dev":          []string{"dev@example.org", "sre@example.org"},
				types.Wildcard: []string{"admin@example.org"},
			},
		},
		{
			desc: "test role_to_recipients but no wildcard",
			in: `
            [mailgun]
            domain = "x"
            private_key = "y"
            [delivery]
            sender = "email@example.org"

			[role_to_recipients]
			"dev" = ["dev@example.org","sre@example.org"]
			`,
			expectErr: func(tt require.TestingT, e error, i ...any) {
				require.Error(t, e)
				require.True(t, trace.IsBadParameter(e))
			},
		},
		{
			desc: "test role_to_recipients with wildcard but empty list of recipients",
			in: `
            [mailgun]
            domain = "x"
            private_key = "y"
            [delivery]
            sender = "email@example.org"

			[role_to_recipients]
            "dev" = "email@example.org"
			"*" = []
			`,
			expectErr: func(tt require.TestingT, e error, i ...any) {
				require.Error(t, e)
				require.True(t, trace.IsBadParameter(e))
			},
		},
		{
			desc: "test no recipients or role_to_recipients",
			in: `
            [mailgun]
            domain = "x"
            private_key = "y"
            [delivery]
            sender = "email@example.org"
			`,
			expectErr: func(tt require.TestingT, e error, i ...any) {
				require.Error(t, e)
				require.True(t, trace.IsBadParameter(e))
			},
		},
		{
			desc: "test recipients and role_to_recipients",
			in: `
			[slack]
			token = "token"
			recipients = ["dev@example.org","admin@example.org"]

			[role_to_recipients]
			"dev" = ["dev@example.org","admin@example.org"]
			"*" = "admin@example.org"
			`,
			expectErr: func(tt require.TestingT, e error, i ...any) {
				require.Error(t, e)
				require.True(t, trace.IsBadParameter(e))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "config_test.toml")
			err := os.WriteFile(filePath, []byte(tc.in), 0777)
			require.NoError(t, err)

			c, err := LoadConfig(filePath)
			if tc.expectErr != nil {
				tc.expectErr(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectRecipients, c.RoleToRecipients)
		})
	}
}

func TestSMTPStartTLSPolicy(t *testing.T) {
	for _, tc := range []struct {
		desc           string
		in             string
		expectErr      require.ErrorAssertionFunc
		expectedPolicy mail.StartTLSPolicy
	}{
		{
			desc: "test no policy should fallback to mandatory",
			in: `
			[smtp]
			host = "http://example.org/"
			username = "user1"
			password = "hidden"
			[role_to_recipients]
			"*" = "admin@example.org"
			`,
			expectedPolicy: mail.MandatoryStartTLS,
		},
		{
			desc: "test mandatory policy should return Mandatory policy",
			in: `
			[smtp]
			host = "http://example.org/"
			username = "user1"
			password = "hidden"
			starttls_policy = "mandatory"
			[role_to_recipients]
			"*" = "admin@example.org"
			`,
			expectedPolicy: mail.MandatoryStartTLS,
		},
		{
			desc: "test opportunistic policy should return Opportunistic policy",
			in: `
			[smtp]
			host = "http://example.org/"
			username = "user1"
			password = "hidden"
			starttls_policy = "opportunistic"
			[role_to_recipients]
			"*" = "admin@example.org"
			`,
			expectedPolicy: mail.OpportunisticStartTLS,
		},
		{
			desc: "test disabled policy should return NoStartTLS policy",
			in: `
			[smtp]
			host = "http://example.org/"
			username = "user1"
			password = "hidden"
			starttls_policy = "disabled"
			[role_to_recipients]
			"*" = "admin@example.org"
			`,
			expectedPolicy: mail.NoStartTLS,
		},
		{
			desc: "test invalid policy should return an error",
			in: `
			[smtp]
			host = "http://example.org/"
			username = "user1"
			password = "hidden"
			starttls_policy = "insecure"
			[role_to_recipients]
			"*" = "admin@example.org"
			`,
			expectErr: func(tt require.TestingT, e error, i ...any) {
				require.Error(t, e)
				require.True(t, trace.IsBadParameter(e))
				require.Contains(t, e.Error(), "invalid smtp.starttls_policy")
				require.Contains(t, e.Error(), "mandatory, opportunistic, disabled")
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "config_test.toml")
			err := os.WriteFile(filePath, []byte(tc.in), 0777)
			require.NoError(t, err)

			c, err := LoadConfig(filePath)
			if tc.expectErr != nil {
				tc.expectErr(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedPolicy, c.SMTP.MailStartTLSPolicy)
		})
	}
}
