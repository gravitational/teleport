package tbot

import (
	"testing"

	"github.com/stretchr/testify/require"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

func Test_workloadAttrsForLog(t *testing.T) {
	attrs := &workloadidentityv1.WorkloadAttrs{
		Podman: &workloadidentityv1.WorkloadAttrsPodman{
			Attested: true,
		},
		Sigstore: &workloadidentityv1.WorkloadAttrsSigstore{
			Payloads: []*workloadidentityv1.SigstoreVerificationPayload{
				{Bundle: []byte(`BUNDLE`)},
				{Bundle: []byte(`BUNDLE`)},
			},
		},
	}

	output := workloadAttrsForLog(attrs)
	require.Contains(t, output, "sigstore:{payloads:{count:2}}")
	require.NotContains(t, output, "BUNDLE")
	require.NotNil(t, attrs.Sigstore)
}
