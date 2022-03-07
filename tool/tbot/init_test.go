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

package main

import (
	"testing"

	"github.com/gravitational/teleport/tool/tbot/botfs"
	"github.com/gravitational/teleport/tool/tbot/config"
	"github.com/stretchr/testify/require"
)

func testNoSecureWrite() {

}

func TestInit(t *testing.T) {
	cfg, err := config.NewDefaultConfig("auth.example.com")
	require.NoError(t, err)

	hasACLSupport, err := botfs.HasACLSupport()
	require.NoError(t, err)

	attemptACLs := false
	if hasACLSupport {
		// Test for ACL support first to determine the correct config.
		dir := t.TempDir()

		err := testACL(t.TempDir(), &botfs.ACLOptions{
			
		})
	}
}
