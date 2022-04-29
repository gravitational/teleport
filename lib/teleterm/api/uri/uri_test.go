/*
Copyright 2021 Gravitational, Inc.

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

package uri_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func TestURI(t *testing.T) {
	testCases := []struct {
		in  uri.ResourceURI
		out string
	}{
		{
			uri.NewClusterURI("teleport.sh").AppendServer("server1"),
			"/clusters/teleport.sh/servers/server1",
		},
		{
			uri.NewClusterURI("teleport.sh").AppendApp("app1"),
			"/clusters/teleport.sh/apps/app1",
		},
		{
			uri.NewClusterURI("teleport.sh").AppendDB("dbhost1"),
			"/clusters/teleport.sh/dbs/dbhost1",
		},
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("%v", tt.in), func(t *testing.T) {
			out := tt.in.String()
			if !reflect.DeepEqual(out, tt.out) {
				t.Errorf("out %#v, want %#v", out, tt.out)
			}
		})
	}
}
