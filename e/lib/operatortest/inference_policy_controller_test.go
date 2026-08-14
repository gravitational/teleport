package operatortest

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
)

// TestInferencePolicyController tests the functionality of the Inference
// Policy controller for the Teleport Kubernetes operator. The actual test
// implementations live closer to where the controller is defined in the OSS
// repo, and they are called here so that a client to an enterprise auth server
// can be passed in.
func TestInferencePolicyController(t *testing.T) {
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
			test: testlib.InferencePolicyCreationTest,
		},
		{
			desc: "deletion",
			test: testlib.InferencePolicyDeletionTest,
		},
		{
			desc: "deletion drift",
			test: testlib.InferencePolicyDeletionDriftTest,
		},
		{
			desc: "update",
			test: testlib.InferencePolicyUpdateTest,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			tc.test(t, clt)
		})
	}
}
