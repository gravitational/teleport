package operatortest

import (
	"os"
	"testing"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integrations/operator/controllers/resources/testlib"
	"github.com/gravitational/teleport/lib/modules"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

// TestLoginRuleController tests the functionality of the Login Rule controller
// for the Teleport Kubernetes operator. The actual test implementations live
// closer to where the controller is defined in the OSS repo, and they are
// called here so that a client to an enterprise auth server can be passed in.
func TestLoginRuleController(t *testing.T) {
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
			test: testlib.LoginRuleCreationTest,
		},
		{
			desc: "deletion drift",
			test: testlib.LoginRuleDeletionDriftTest,
		},
		{
			desc: "update",
			test: testlib.LoginRuleUpdateTest,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			tc.test(t, clt)
		})
	}
}
