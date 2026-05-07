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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderText(t *testing.T) {
	t.Parallel()

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
			desc: "run failure",
			instances: []instanceInfo{
				{
					AWS:      &awsInfo{InstanceID: "i-ssm001", AccountID: "111111111111"},
					Region:   "us-east-1",
					IsOnline: false,
					RunResult: &runResult{
						ExitCode:  1,
						Output:    "install script failed",
						Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
			},
			want: `Cloud Account       Region    Instance Time          Status        Details      
----- ------------- --------- -------- ------------- ------------- ------------ 
AWS   1111111111... us-east-1 i-ssm001 2026-01-15... Failed (ex... Script ou... 
`,
		},
		{
			desc: "online instance with no run result",
			instances: []instanceInfo{
				{
					AWS:      &awsInfo{InstanceID: "i-ok001", AccountID: "222222222222"},
					Region:   "eu-west-1",
					IsOnline: true,
					Expiry:   time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				},
			},
			want: `Cloud Account       Region    Instance Time          Status Details 
----- ------------- --------- -------- ------------- ------ ------- 
AWS   2222222222... eu-west-1 i-ok001  2026-01-15... Online         
`,
		},
		{
			desc: "success with output",
			instances: []instanceInfo{
				{
					AWS:      &awsInfo{InstanceID: "i-ok002", AccountID: "444444444444"},
					Region:   "us-west-2",
					IsOnline: true,
					RunResult: &runResult{
						ExitCode: 0,
						Output:   "installed successfully",
						Time:     time.Date(2026, 1, 15, 11, 30, 0, 0, time.UTC),
					},
				},
			},
			want: `Cloud Account       Region    Instance Time          Status Details             
----- ------------- --------- -------- ------------- ------ ------------------- 
AWS   4444444444... us-west-2 i-ok002  2026-01-15... Online Script output: "... 
`,
		},
		{
			desc: "mixed instances",
			instances: []instanceInfo{
				{
					AWS:      &awsInfo{InstanceID: "i-ok001", AccountID: "222222222222"},
					Region:   "eu-west-1",
					IsOnline: true,
					Expiry:   time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
				},
				{
					AWS:      &awsInfo{InstanceID: "i-ssm001", AccountID: "111111111111"},
					Region:   "us-east-1",
					IsOnline: false,
					RunResult: &runResult{
						ExitCode:  1,
						Output:    "install script failed",
						Time:      time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
				{
					AWS:      &awsInfo{InstanceID: "i-ssm002", AccountID: "333333333333"},
					Region:   "ap-south-1",
					IsOnline: true,
					Expiry:   time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
					RunResult: &runResult{
						ExitCode:  2,
						Output:    "permission denied",
						Time:      time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
						IsFailure: true,
					},
				},
			},
			want: `Cloud Account       Region     Instance Time          Status        Details     
----- ------------- ---------- -------- ------------- ------------- ----------- 
AWS   2222222222... eu-west-1  i-ok001  2026-01-15... Online                    
AWS   1111111111... us-east-1  i-ssm001 2026-01-15... Failed (ex... Script o... 
AWS   3333333333... ap-south-1 i-ssm002 2026-01-15... Online, ex... Script o... 
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
