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
