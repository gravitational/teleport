// Copyright 2023 Gravitational, Inc
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

// Package kubeconfig manages teleport entries in a local kubeconfig file.
package kubeconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckContextOverrideTemplate(t *testing.T) {
	type args struct {
		temp string
	}
	tests := []struct {
		name        string
		args        args
		assertErr   require.ErrorAssertionFunc
		errContains string
	}{
		{
			name: "empty template",
			args: args{
				temp: "",
			},
			assertErr: require.NoError,
		},
		{
			name: "valid template",
			args: args{
				temp: "{{ .KubeName }}-{{ .ClusterName }}",
			},
			assertErr: require.NoError,
		},
		{
			name: "invalid template",
			args: args{
				temp: "{{ .KubeName2 }}-{{ .ClusterName }}",
			},
			assertErr:   require.Error,
			errContains: "failed to parse context override template",
		},
		{
			name: "invalid template",
			args: args{
				temp: "{{ .ClusterName }}",
			},
			assertErr:   require.Error,
			errContains: "using the same context override template for different clusters is not allowed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckContextOverrideTemplate(tt.args.temp)
			tt.assertErr(t, err)
			if err != nil {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}
