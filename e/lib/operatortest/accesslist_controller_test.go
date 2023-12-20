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

package operatortest

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

// TestAccessListController tests the functionality of the AccessList controller
// for the Teleport Kubernetes operator. The actual test implementations live
// closer to where the controller is defined in the OSS repo, and they are
// called here so that a client to an enterprise auth server can be passed in.
func TestAccessListController(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("skipping operator test because KUBEBUILDER_ASSETS env var is missing")
	}

	// These tests can't be parallelized because testlib.SetupTestEnv modifies
	// the global scheme causing a data race.
	// t.Parallel()

	clt := startAuthServer(t)

	for _, tc := range []struct {
		desc string
		test func(*testing.T, *client.Client)
	}{
		{
			desc: "creation",
			test: testlib.AccessListCreationTest,
		},
		{
			desc: "deletion drift",
			test: testlib.AccessListDeletionDriftTest,
		},
		{
			desc: "update",
			test: testlib.AccessListUpdateTest,
		},
		{
			desc: "mutate",
			test: testlib.AccessListMutateExistingTest,
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			tc.test(t, clt)
		})
	}
}
