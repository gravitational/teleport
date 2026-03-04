// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderText(t *testing.T) {
	tests := []struct {
		desc      string
		instances []instanceInfo
		want      string
	}{
		{
			desc:      "empty input shows no-instances message",
			instances: nil,
			want: `No instances found.
`,
		},
		{
			desc: "SSM failure",
			instances: []instanceInfo{
				{
					InstanceID: "i-ssm001",
					Region:     "us-east-1",
					AccountID:  "111111111111",
					IsOnline:   false,
					SSMResult: &ssmResult{
						ExitCode:  1,
						Output:    "install script failed",
						Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
			},
			want: `Instance ID Region    Account      Online Time                 Result SSM Output              
----------- --------- ------------ ------ -------------------- ------ ----------------------- 
i-ssm001    us-east-1 111111111111 no     2026-01-15T10:00:00Z exit=1 "install script failed" 
`,
		},
		{
			desc: "online instance with no SSM result",
			instances: []instanceInfo{
				{
					InstanceID: "i-ok001",
					Region:     "eu-west-1",
					AccountID:  "222222222222",
					IsOnline:   true,
					Expiry:     time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				},
			},
			want: `Instance ID Region    Account      Online Time                 Result SSM Output 
----------- --------- ------------ ------ -------------------- ------ ---------- 
i-ok001     eu-west-1 222222222222 yes    2026-01-15T11:00:00Z                   
`,
		},
		{
			desc: "SSM success with output",
			instances: []instanceInfo{
				{
					InstanceID: "i-ok002",
					Region:     "us-west-2",
					AccountID:  "444444444444",
					IsOnline:   true,
					SSMResult: &ssmResult{
						ExitCode: 0,
						Output:   "installed successfully",
						Time:     time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC),
					},
				},
			},
			want: `Instance ID Region    Account      Online Time                 Result SSM Output               
----------- --------- ------------ ------ -------------------- ------ ------------------------ 
i-ok002     us-west-2 444444444444 yes    2026-01-15T11:30:00Z exit=0 "installed successfully" 
`,
		},
		{
			desc: "mixed instances",
			instances: []instanceInfo{
				{
					InstanceID: "i-ok001",
					Region:     "eu-west-1",
					AccountID:  "222222222222",
					IsOnline:   true,
					Expiry:     time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				},
				{
					InstanceID: "i-ssm001",
					Region:     "us-east-1",
					AccountID:  "111111111111",
					IsOnline:   false,
					SSMResult: &ssmResult{
						ExitCode:  1,
						Output:    "install script failed",
						Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
				{
					InstanceID: "i-ssm002",
					Region:     "ap-south-1",
					AccountID:  "333333333333",
					IsOnline:   true,
					Expiry:     time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
					SSMResult: &ssmResult{
						ExitCode:  2,
						Output:    "permission denied",
						Time:      time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
			},
			want: `Instance ID Region     Account      Online Time                 Result SSM Output              
----------- ---------- ------------ ------ -------------------- ------ ----------------------- 
i-ok001     eu-west-1  222222222222 yes    2026-01-15T11:00:00Z                                
i-ssm001    us-east-1  111111111111 no     2026-01-15T10:00:00Z exit=1 "install script failed" 
i-ssm002    ap-south-1 333333333333 yes    2026-01-15T12:00:00Z exit=2 "permission denied"     
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, renderText(&buf, tt.instances))
			require.Equal(t, tt.want, buf.String())
		})
	}
}

func TestRenderJSON(t *testing.T) {
	tests := []struct {
		desc      string
		instances []instanceInfo
		want      []instanceInfo
	}{
		{
			desc: "round-trip preserves all fields",
			instances: []instanceInfo{
				{
					InstanceID: "i-ssm001",
					Region:     "us-east-1",
					AccountID:  "111111111111",
					IsOnline:   false,
					SSMResult: &ssmResult{
						ExitCode:  1,
						Output:    "install script failed",
						Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
				{
					InstanceID: "i-ok001",
					Region:     "eu-west-1",
					AccountID:  "222222222222",
					IsOnline:   true,
					Expiry:     time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				},
			},
			want: []instanceInfo{
				{
					InstanceID: "i-ssm001",
					Region:     "us-east-1",
					AccountID:  "111111111111",
					SSMResult: &ssmResult{
						ExitCode:  1,
						Output:    "install script failed",
						Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
				{
					InstanceID: "i-ok001",
					Region:     "eu-west-1",
					AccountID:  "222222222222",
					IsOnline:   true,
					Expiry:     time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			desc:      "nil produces empty JSON array",
			instances: nil,
			want:      []instanceInfo{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, renderJSON(&buf, tt.instances))

			var got []instanceInfo
			require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
			require.Equal(t, tt.want, got)
		})
	}
}
