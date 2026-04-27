package operatortest

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

// TestRetrievalModelController tests the functionality of the RetrievalModel
// controller for the Teleport Kubernetes operator. The actual test
// implementations live closer to where the controller is defined in the OSS
// repo, and they are called here so that a client to an enterprise auth server
// can be passed in.
// RetrievalModel is a singleton resource so these tests must be run sequentially,
// and they cannot be run in parallel.
func TestRetrievalModelController(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("skipping operator test because KUBEBUILDER_ASSETS env var is missing")
	}

	clt := startAuthServer(t)

	for _, tc := range []struct {
		desc string
		test func(*testing.T, *client.Client)
	}{
		{
			desc: "creation",
			test: testlib.RetrievalModelCreationTest,
		},
		{
			desc: "deletion",
			test: testlib.RetrievalModelDeletionTest,
		},
		{
			desc: "deletion drift",
			test: testlib.RetrievalModelDeletionDriftTest,
		},
		{
			desc: "update",
			test: testlib.RetrievalModelUpdateTest,
		},
		{
			desc: "wrong name",
			test: testlib.RetrievalModelWrongNameTest,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			tc.test(t, clt)
		})
	}
}
