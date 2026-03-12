package identitycenter

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/common"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	"github.com/gravitational/teleport/lib/services"
)

func TestAccountStartURL(t *testing.T) {
	const (
		name       = "account1"
		id         = services.IdentityCenterAccountID("account1")
		instanceID = "d-9268xxxxxc"
		idSource   = icsdk.IdentityStoreID(instanceID)
	)

	testCases := []struct {
		name        string
		arn         string
		urlContains string
	}{

		{
			name:        "gov cloud account",
			arn:         "arn:aws-us-gov:organizations::190000000076:account/o-boxxxxxxxp/060000000001",
			urlContains: "https://start.us-gov-home",
		},
		{
			name:        "commercial account",
			arn:         "arn:aws:organizations::02000000032:account/o-uxxxxxxxph/110000000005",
			urlContains: `https://` + instanceID + `.awsapps.com/start`,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			arn, err := arn.Parse(test.arn)
			require.NoError(t, err)
			account := newIdentityCenterAccount(name, id, arn, idSource, "us-east-1")
			require.NotNil(t, account)
			require.NotEmpty(t, account.Spec.StartUrl)
			require.Contains(t, account.Spec.StartUrl, test.urlContains)
			require.Equal(t, map[string]string{
				types.OriginLabel:         common.OriginAWSIdentityCenter,
				types.AWSAccountIDLabel:   string(id),
				types.AWSAccountNameLabel: name,
				types.AWSSSORegionLabel:   "us-east-1",
			}, account.GetMetadata().GetLabels())
		})
	}
}
