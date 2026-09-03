/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsoleURLPartition(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		wantPartition string
		wantOK        bool
	}{
		{
			name:          "standard console",
			rawURL:        "https://console.aws.amazon.com/ec2/v2/home?region=us-east-1#Instances:",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "standard console uppercase host",
			rawURL:        "https://CONSOLE.AWS.AMAZON.COM/",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "standard console trailing dot host",
			rawURL:        "https://console.aws.amazon.com./",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "standard console explicit HTTPS port",
			rawURL:        "https://console.aws.amazon.com:443/",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "standard console uppercase host explicit HTTPS port",
			rawURL:        "https://CONSOLE.AWS.AMAZON.COM:443/",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "regional standard console",
			rawURL:        "https://us-west-1.console.aws.amazon.com/ec2/v2/home",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "regional standard console uppercase region",
			rawURL:        "https://US-WEST-1.console.aws.amazon.com/",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "quicksight",
			rawURL:        "https://quicksight.aws.amazon.com/sn/start",
			wantPartition: StandardPartition,
			wantOK:        true,
		},
		{
			name:          "govcloud",
			rawURL:        "https://console.amazonaws-us-gov.com/console/home",
			wantPartition: USGovPartition,
			wantOK:        true,
		},
		{
			name:          "china",
			rawURL:        "https://console.amazonaws.cn/console/home",
			wantPartition: CNPartition,
			wantOK:        true,
		},
		{
			name:   "userinfo host spoof",
			rawURL: "https://console.aws.amazon.com@attacker.example/",
		},
		{
			name:   "regional userinfo host spoof",
			rawURL: "https://us-west-1.console.aws.amazon.com@attacker.example/",
		},
		{
			name:   "prefix host spoof",
			rawURL: "https://console.aws.amazon.com.evil.example/",
		},
		{
			name:   "regional prefix host spoof",
			rawURL: "https://us-west-1.console.aws.amazon.com.evil.example/",
		},
		{
			name:   "console URL in query",
			rawURL: "https://attacker.example/?next=https://console.aws.amazon.com/",
		},
		{
			name:   "non-HTTPS",
			rawURL: "http://console.aws.amazon.com/",
		},
		{
			name:   "unexpected port",
			rawURL: "https://console.aws.amazon.com:444/",
		},
		{
			name:   "malformed port",
			rawURL: "https://console.aws.amazon.com:bad/",
		},
		{
			name:   "invalid regional console subdomain",
			rawURL: "https://evil.console.aws.amazon.com/",
		},
		{
			name:   "unexpected nested regional console subdomain",
			rawURL: "https://us-west-1.evil.console.aws.amazon.com/",
		},
		{
			name:   "GovCloud region on standard console suffix",
			rawURL: "https://us-gov-west-1.console.aws.amazon.com/",
		},
		{
			name:   "China region on standard console suffix",
			rawURL: "https://cn-north-1.console.aws.amazon.com/",
		},
		{
			name:   "AWS global pseudo-region on standard console suffix",
			rawURL: "https://aws-global.console.aws.amazon.com/",
		},
		{
			name:   "malformed URL",
			rawURL: "://console.aws.amazon.com/",
		},
		{
			name: "empty URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotPartition, gotOK := ConsoleURLPartition(test.rawURL)
			require.Equal(t, test.wantOK, gotOK)
			require.Equal(t, test.wantPartition, gotPartition)
			require.Equal(t, test.wantOK, IsConsoleURL(test.rawURL))
		})
	}
}
