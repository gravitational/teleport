/*
Copyright 2023 Gravitational, Inc.

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

package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatLocalCommandString(t *testing.T) {
	var tests = []struct {
		TSHPath        string
		clusterName    string
		expectedOutput string
	}{
		{
			TSHPath:        string{`C:\Users\Test\tsh.exe`},
			clusterName:    string{`teleport.example.com`},
			expectedOutput: string{"C:\\Users\\Test\\tsh.exe proxy ssh --cluster=teleport.example.com --proxy=%proxyhost %user@%host:%port"},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			output := FormatLocalCommandString(tt.TSHPath, tt.clusterName)
			require.ElementsMatch(t, tt.expected, output)
		})
	}
}
