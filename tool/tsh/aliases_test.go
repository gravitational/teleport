// Copyright 2022 Gravitational, Inc
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

package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_expandAliasDefinition(t *testing.T) {
	tests := []struct {
		name      string
		aliasDef  string
		argsGiven []string
		want      []string
		wantErr   bool
	}{
		{
			name:      "empty",
			aliasDef:  "",
			argsGiven: nil,
			want:      []string{},
			wantErr:   false,
		},
		{
			name:      "empty alias, append all",
			aliasDef:  "",
			argsGiven: []string{"foo", "bar", "baz"},
			want:      []string{"foo", "bar", "baz"},
			wantErr:   false,
		},
		{
			name:      "append unused elems",
			aliasDef:  "$3 $0",
			argsGiven: []string{"arg0", "arg1", "arg2", "arg3"},
			want:      []string{"arg3", "arg0", "arg1", "arg2"},
			wantErr:   false,
		},
		{
			name:      "no references, append args",
			aliasDef:  "foo1 foo2 foo3",
			argsGiven: []string{"arg1", "arg2", "arg3"},
			want:      []string{"foo1", "foo2", "foo3", "arg1", "arg2", "arg3"},
			wantErr:   false,
		},
		{
			name:      "valid references",
			aliasDef:  "refFoo $0 refBar $1",
			argsGiven: []string{"foo", "bar"},
			want:      []string{"refFoo", "foo", "refBar", "bar"},
			wantErr:   false,
		},
		{
			name:      "out of range reference",
			aliasDef:  "refFoo $0 refBar $1",
			argsGiven: []string{"foo"},
			wantErr:   true,
		},
		{
			name:      "ignore negative reference",
			aliasDef:  "$-100",
			argsGiven: nil,
			want:      []string{"$-100"},
			wantErr:   false,
		},
		{
			name:      "unknown references fail",
			argsGiven: nil,
			aliasDef:  "$100 200 300",
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandAliasDefinition(tt.aliasDef, tt.argsGiven)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_findCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantAlias string
		wantIndex int
	}{
		{
			name:      "empty args not found",
			args:      nil,
			wantAlias: "",
			wantIndex: -1,
		},
		{
			name:      "only options, not found",
			args:      []string{"--foo", "--bar", "-baz", "--"},
			wantAlias: "",
			wantIndex: -1,
		},
		{
			name:      "first place",
			args:      []string{"login", "--foo", "--bar"},
			wantAlias: "login",
			wantIndex: 0,
		},
		{
			name:      "second place",
			args:      []string{"--foo", "login", "--bar"},
			wantAlias: "login",
			wantIndex: 1,
		},
		{
			name:      "last place",
			args:      []string{"--foo", "--bar", "login"},
			wantAlias: "login",
			wantIndex: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, i := findCommand(tt.args)
			require.Equal(t, tt.wantAlias, a)
			require.Equal(t, tt.wantIndex, i)
		})
	}
}

func Test_getSeenAliases(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "empty",
			env:  nil,
			want: nil,
		},
		{
			name: "commas",
			env:  map[string]string{tshAliasEnvKey: ",,,"},
			want: nil,
		},
		{
			name: "few values",
			env:  map[string]string{tshAliasEnvKey: "foo,bar,baz,,,"},
			want: []string{"foo", "bar", "baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			getEnv := func(key string) string {
				val := tt.env[key]
				return val
			}

			require.Equal(t, tt.want, getSeenAliases(getEnv))
		})
	}
}
