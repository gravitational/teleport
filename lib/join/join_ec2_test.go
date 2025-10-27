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

package join_test

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/join/ec2join"
	"github.com/gravitational/teleport/lib/join/joinclient"
)

type ec2Instance struct {
	iid         []byte
	account     string
	region      string
	instanceID  string
	pendingTime time.Time
}

var (
	instance1 = ec2Instance{
		iid: []byte(`MIAGCSqGSIb3DQEHAqCAMIACAQExDzANBglghkgBZQMEAgEFADCABgkqhkiG9w0BBwGggCSABIIB
23sKICAiYWNjb3VudElkIiA6ICIyNzg1NzYyMjA0NTMiLAogICJhcmNoaXRlY3R1cmUiIDogIng4
Nl82NCIsCiAgImF2YWlsYWJpbGl0eVpvbmUiIDogInVzLXdlc3QtMmEiLAogICJiaWxsaW5nUHJv
ZHVjdHMiIDogbnVsbCwKICAiZGV2cGF5UHJvZHVjdENvZGVzIiA6IG51bGwsCiAgIm1hcmtldHBs
YWNlUHJvZHVjdENvZGVzIiA6IG51bGwsCiAgImltYWdlSWQiIDogImFtaS0wZmE5ZTFmNjQxNDJj
ZGUxNyIsCiAgImluc3RhbmNlSWQiIDogImktMDc4NTE3Y2E4YTcwYTFkZGUiLAogICJpbnN0YW5j
ZVR5cGUiIDogInQyLm1lZGl1bSIsCiAgImtlcm5lbElkIiA6IG51bGwsCiAgInBlbmRpbmdUaW1l
IiA6ICIyMDIxLTA5LTAzVDIxOjI1OjQ0WiIsCiAgInByaXZhdGVJcCIgOiAiMTAuMC4wLjIwOSIs
CiAgInJhbWRpc2tJZCIgOiBudWxsLAogICJyZWdpb24iIDogInVzLXdlc3QtMiIsCiAgInZlcnNp
b24iIDogIjIwMTctMDktMzAiCn0AAAAAAAAxggIvMIICKwIBATBpMFwxCzAJBgNVBAYTAlVTMRkw
FwYDVQQIExBXYXNoaW5ndG9uIFN0YXRlMRAwDgYDVQQHEwdTZWF0dGxlMSAwHgYDVQQKExdBbWF6
b24gV2ViIFNlcnZpY2VzIExMQwIJALZL3lrQCSTMMA0GCWCGSAFlAwQCAQUAoIGYMBgGCSqGSIb3
DQEJAzELBgkqhkiG9w0BBwEwHAYJKoZIhvcNAQkFMQ8XDTIxMDkwMzIxMjU0N1owLQYJKoZIhvcN
AQk0MSAwHjANBglghkgBZQMEAgEFAKENBgkqhkiG9w0BAQsFADAvBgkqhkiG9w0BCQQxIgQgCH2d
JiKmdx9uhxlm8ObWAvFOhqJb7k79+DW/T3ezwVUwDQYJKoZIhvcNAQELBQAEggEANWautigs/qZ6
w8g5/EfWsAFj8kHgUD+xqsQ1HDrBUx3IQ498NMBZ78379B8RBfuzeVjbaf+yugov0fYrDbGvSRRw
myy49TfZ9gdlpWQXzwSg3OPMDNToRoKw00/LQjSxcTCaPP4vMDEIjYMUqZ3i4uWYJJJ0Lb7fDMDk
Anu7yHolVfbnvIAuZe8lGpc7ofCSBG5wulm+/pqzO25YPMH1cLEvOadE+3N2GxK6gRTLJoE98rsm
LDp6OuU/b2QfaxU0ec6OogdtSJto/URI0/ygHmNAzBis470A29yh5nVwm6AkY4krjPsK7uiBIRhs
lr5x0X6+ggQfF2BKAJ/BRcAHNgAAAAAAAA==`),
		account:     "278576220453",
		region:      "us-west-2",
		instanceID:  "i-078517ca8a70a1dde",
		pendingTime: time.Date(2021, time.September, 3, 21, 25, 44, 0, time.UTC),
	}
	instance2 = ec2Instance{
		iid: []byte(`MIAGCSqGSIb3DQEHAqCAMIACAQExDzANBglghkgBZQMEAgEFADCABgkqhkiG9w0BBwGggCSABIIB
3XsKICAiYWNjb3VudElkIiA6ICI4ODM0NzQ2NjI4ODgiLAogICJhcmNoaXRlY3R1cmUiIDogIng4
Nl82NCIsCiAgImF2YWlsYWJpbGl0eVpvbmUiIDogInVzLXdlc3QtMWMiLAogICJiaWxsaW5nUHJv
ZHVjdHMiIDogbnVsbCwKICAiZGV2cGF5UHJvZHVjdENvZGVzIiA6IG51bGwsCiAgIm1hcmtldHBs
YWNlUHJvZHVjdENvZGVzIiA6IG51bGwsCiAgImltYWdlSWQiIDogImFtaS0wY2UzYzU1YTMxZDI5
MDQwZSIsCiAgImluc3RhbmNlSWQiIDogImktMDFiOTQwYzQ1ZmQxMWZlNzQiLAogICJpbnN0YW5j
ZVR5cGUiIDogInQyLm1pY3JvIiwKICAia2VybmVsSWQiIDogbnVsbCwKICAicGVuZGluZ1RpbWUi
IDogIjIwMjEtMDktMTFUMDA6MTQ6MThaIiwKICAicHJpdmF0ZUlwIiA6ICIxNzIuMzEuMTIuMjUx
IiwKICAicmFtZGlza0lkIiA6IG51bGwsCiAgInJlZ2lvbiIgOiAidXMtd2VzdC0xIiwKICAidmVy
c2lvbiIgOiAiMjAxNy0wOS0zMCIKfQAAAAAAADGCAi8wggIrAgEBMGkwXDELMAkGA1UEBhMCVVMx
GTAXBgNVBAgTEFdhc2hpbmd0b24gU3RhdGUxEDAOBgNVBAcTB1NlYXR0bGUxIDAeBgNVBAoTF0Ft
YXpvbiBXZWIgU2VydmljZXMgTExDAgkA00+QilzIS0gwDQYJYIZIAWUDBAIBBQCggZgwGAYJKoZI
hvcNAQkDMQsGCSqGSIb3DQEHATAcBgkqhkiG9w0BCQUxDxcNMjEwOTExMDAxNDIyWjAtBgkqhkiG
9w0BCTQxIDAeMA0GCWCGSAFlAwQCAQUAoQ0GCSqGSIb3DQEBCwUAMC8GCSqGSIb3DQEJBDEiBCDS
1gNvxbYnEL6plVu8X/QmKPJFJwIJfi+2hIVjyKAOtjANBgkqhkiG9w0BAQsFAASCAQABKmghATg8
VXkdiIGcTIPfKrc2v/zEIdLUAi+Ew5lrGUVjnNqrP9irGK4d9sVtcu/8UKp9RDoeJOQ6I/pRcwvT
PJVHlhGnLyybr5ZVqkxiC09GASNnPe12dzCKkKD2rvW6mGR91cxpM94Xqi5UA/ZRqiXwpHo3LECN
Gu38Hpdv6sBgD/av2Ohd+vEH2zvYVkp7ZfnFuDLWRSBQZgmKwVKVdOjrMmP32vb3vzhMBuOj+jbQ
RwEmYIkRaEGNbrZgatjMJYmTWuLG26zws3avOK6kL6u38DV3wJPVB/G0Ira5MvC/ojGya+DrVngW
VUP+3jgenPrd7OyCWPSwOoOBMhSlAAAAAAAA`),
		account:     "883474662888",
		region:      "us-west-1",
		instanceID:  "i-01b940c45fd11fe74",
		pendingTime: time.Date(2021, time.September, 11, 0, 14, 18, 0, time.UTC),
	}
)

type (
	ec2ClientNoInstance struct{}
	ec2ClientNotRunning struct{}
	ec2ClientRunning    struct{}
)

func (c ec2ClientNoInstance) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}

func (c ec2ClientNotRunning) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{
				Instances: []ec2types.Instance{
					{
						InstanceId: &params.InstanceIds[0],
						State: &ec2types.InstanceState{
							Name: ec2types.InstanceStateNameTerminated,
						},
					},
				},
			},
		},
	}, nil
}

func (c ec2ClientRunning) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{
				Instances: []ec2types.Instance{
					{
						InstanceId: &params.InstanceIds[0],
						State: &ec2types.InstanceState{
							Name: ec2types.InstanceStateNameRunning,
						},
					},
				},
			},
		},
	}, nil
}

func TestJoinEC2(t *testing.T) {
	ctx := context.Background()

	testServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)

	// upsert a node to test duplicates
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      instance2.account + "-" + instance2.instanceID,
			Namespace: defaults.Namespace,
		},
	}
	_, err = testServer.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	nopClient, err := testServer.NewClient(authtest.TestNop())
	require.NoError(t, err)

	isAccessDenied := func(t require.TestingT, err error, args ...any) {
		if helper, ok := t.(interface{ Helper() }); ok {
			helper.Helper()
		}
		require.ErrorAs(t, err, new(*trace.AccessDeniedError), args...)
	}

	badInstanceId := instance1.account + "-i-99999999"

	testCases := []struct {
		desc          string
		tokenSpec     types.ProvisionTokenSpecV2
		ec2Client     ec2join.EC2Client
		document      []byte
		requestHostID string
		expectError   require.ErrorAssertionFunc
		clock         clockwork.Clock
	}{
		{
			desc: "basic passing case",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   require.NoError,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "pass with multiple rules",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance2.account,
						AWSRegions: []string{instance2.region},
					},
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   require.NoError,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "pass with multiple regions",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{"us-east-1", instance1.region, "us-east-2"},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   require.NoError,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "pass with no regions",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   require.NoError,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "wrong account",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "bad account",
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "wrong region",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{"bad region"},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "bad HostID",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: badInstanceId,
			document:      instance1.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "no identity document",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "bad identity document",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      []byte("baddocument"),
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "instance already joined",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance2.account,
						AWSRegions: []string{instance2.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance2.account + "-" + instance2.instanceID,
			document:      instance2.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance2.pendingTime),
		},
		{
			desc: "instance already joined, fake ID",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance2.account,
						AWSRegions: []string{instance2.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance2.account + "-" + instance2.instanceID,
			document:      instance2.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance2.pendingTime),
		},
		{
			desc: "instance not running",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientNotRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "instance not exists",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientNoInstance{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime),
		},
		{
			desc: "TTL expired",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime.Add(5*time.Minute + time.Second)),
		},
		{
			desc: "custom TTL pass",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
				AWSIIDTTL: types.Duration(10 * time.Minute),
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   require.NoError,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime.Add(9 * time.Minute)),
		},
		{
			desc: "custom TTL fail",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: instance1.account,
						AWSRegions: []string{instance1.region},
					},
				},
				AWSIIDTTL: types.Duration(10 * time.Minute),
			},
			ec2Client:     ec2ClientRunning{},
			requestHostID: instance1.account + "-" + instance1.instanceID,
			document:      instance1.iid,
			expectError:   isAccessDenied,
			clock:         clockwork.NewFakeClockAt(instance1.pendingTime.Add(11 * time.Minute)),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			clock := tc.clock
			if clock == nil {
				clock = clockwork.NewRealClock()
			}
			testServer.Auth().SetClock(clock)

			token, err := types.NewProvisionTokenFromSpec(
				"test_token",
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)

			err = testServer.Auth().UpsertToken(context.Background(), token)
			require.NoError(t, err)

			testServer.Auth().SetEC2ClientForEC2JoinMethod(tc.ec2Client)

			t.Run("new", func(t *testing.T) {
				if tc.requestHostID == badInstanceId {
					// New join method does not allow the client so request a
					// specific host ID, so the join would pass and fail the
					// error assertion.
					t.Skip()
				}
				_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
					Token: "test_token",
					ID: state.IdentityID{
						Role:     types.RoleInstance,
						NodeName: "testnode",
					},
					AuthClient: nopClient,
					GetInstanceIdentityDocumentFunc: func(_ context.Context) ([]byte, error) {
						return tc.document, nil
					},
				})
				tc.expectError(t, err)
			})
			t.Run("legacy", func(t *testing.T) {
				_, err = joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
					Token:      "test_token",
					JoinMethod: types.JoinMethodEC2,
					ID: state.IdentityID{
						Role:     types.RoleNode,
						NodeName: "testnode",
						HostUUID: tc.requestHostID,
					},
					AuthClient: nopClient,
					GetInstanceIdentityDocumentFunc: func(_ context.Context) ([]byte, error) {
						return tc.document, nil
					},
				})
				tc.expectError(t, err)
			})

			err = testServer.Auth().DeleteToken(context.Background(), token.GetName())
			require.NoError(t, err)
		})
	}
}

// TestHostUniqueCheck tests the uniqueness check used by checkEC2JoinRequest
func TestHostUniqueCheck(t *testing.T) {
	testServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	a := testServer.Auth()

	token, err := types.NewProvisionTokenFromSpec(
		"test_token",
		time.Now().Add(time.Minute),
		types.ProvisionTokenSpecV2{
			Roles: []types.SystemRole{
				types.RoleNode,
				types.RoleProxy,
				types.RoleKube,
				types.RoleDatabase,
				types.RoleApp,
				types.RoleWindowsDesktop,
				types.RoleMDM,
				types.RoleDiscovery,
				types.RoleOkta,
			},
			Allow: []*types.TokenRule{
				{
					AWSAccount: instance1.account,
					AWSRegions: []string{instance1.region},
				},
			},
		})
	require.NoError(t, err)

	err = a.UpsertToken(context.Background(), token)
	require.NoError(t, err)

	testCases := []struct {
		role     types.SystemRole
		upserter func(t *testing.T, hostID string)
		deleter  func(t *testing.T, hostID string)
	}{
		{
			role: types.RoleNode,
			upserter: func(t *testing.T, hostID string) {
				node := &types.ServerV2{
					Kind:    types.KindNode,
					Version: types.V2,
					Metadata: types.Metadata{
						Name:      hostID,
						Namespace: defaults.Namespace,
					},
				}
				_, err := a.UpsertNode(context.Background(), node)
				require.NoError(t, err)
			},
			deleter: func(t *testing.T, hostID string) {
				require.NoError(t, a.DeleteNode(t.Context(), defaults.Namespace, hostID))
			},
		},
		{
			role: types.RoleProxy,
			upserter: func(t *testing.T, hostID string) {
				proxy := &types.ServerV2{
					Kind:    types.KindProxy,
					Version: types.V2,
					Metadata: types.Metadata{
						Name:      hostID,
						Namespace: defaults.Namespace,
					},
				}
				err := a.UpsertProxy(context.Background(), proxy)
				require.NoError(t, err)
			},
			deleter: func(t *testing.T, hostID string) {
				require.NoError(t, a.DeleteProxy(t.Context(), hostID))
			},
		},
		{
			role: types.RoleKube,
			upserter: func(t *testing.T, hostID string) {
				kube, err := types.NewKubernetesServerV3(
					types.Metadata{
						Name:      "test-kube-cluster",
						Namespace: defaults.Namespace,
					},
					types.KubernetesServerSpecV3{
						HostID:   hostID,
						Hostname: "test-kube-hostname",
						Cluster: &types.KubernetesClusterV3{
							Metadata: types.Metadata{
								Name:      "test-kube-cluster",
								Namespace: defaults.Namespace,
							},
						},
					})
				require.NoError(t, err)
				_, err = a.UpsertKubernetesServer(context.Background(), kube)
				require.NoError(t, err)
			},
			deleter: func(t *testing.T, hostID string) {
				require.NoError(t, a.DeleteKubernetesServer(t.Context(), hostID, "test-kube-cluster"))
			},
		},
		{
			role: types.RoleDatabase,
			upserter: func(t *testing.T, hostID string) {
				db, err := types.NewDatabaseServiceV1(
					types.Metadata{
						Name: hostID,
					},
					types.DatabaseServiceSpecV1{},
				)
				require.NoError(t, err)
				_, err = a.UpsertDatabaseService(context.Background(), db)
				require.NoError(t, err)
			},
			deleter: func(t *testing.T, hostID string) {
				require.NoError(t, a.DeleteDatabaseService(t.Context(), hostID))
			},
		},
		{
			role: types.RoleApp,
			upserter: func(t *testing.T, hostID string) {
				app, err := types.NewAppV3(
					types.Metadata{
						Name:      "test-app",
						Namespace: defaults.Namespace,
					},
					types.AppSpecV3{
						URI: "https://app.localhost",
					})
				require.NoError(t, err)
				appServer, err := types.NewAppServerV3(
					types.Metadata{
						Name:      "test-app",
						Namespace: defaults.Namespace,
					},
					types.AppServerSpecV3{
						HostID: hostID,
						App:    app,
					})
				require.NoError(t, err)
				_, err = a.UpsertApplicationServer(context.Background(), appServer)
				require.NoError(t, err)
			},
			deleter: func(t *testing.T, hostID string) {
				require.NoError(t, a.DeleteApplicationServer(t.Context(), defaults.Namespace, hostID, "test-app"))
			},
		},
		{
			role: types.RoleWindowsDesktop,
			upserter: func(t *testing.T, hostID string) {
				wds, err := types.NewWindowsDesktopServiceV3(types.Metadata{Name: hostID},
					types.WindowsDesktopServiceSpecV3{
						Addr:            "localhost:3028",
						TeleportVersion: "10.2.2",
					})
				require.NoError(t, err)

				_, err = a.UpsertWindowsDesktopService(context.Background(), wds)
				require.NoError(t, err)
			},
			deleter: func(t *testing.T, hostID string) {
				require.NoError(t, a.DeleteWindowsDesktopService(t.Context(), hostID))
			},
		},
		{
			role: types.RoleOkta,
			upserter: func(t *testing.T, hostID string) {
				app, err := types.NewAppV3(
					types.Metadata{
						Name:      "test-okta-app",
						Namespace: defaults.Namespace,
					},
					types.AppSpecV3{
						URI: "https://app.localhost",
					})
				require.NoError(t, err)
				appServer, err := types.NewAppServerV3(
					types.Metadata{
						Name:      "test-okta-app",
						Namespace: defaults.Namespace,
					},
					types.AppServerSpecV3{
						HostID: hostID,
						App:    app,
					})
				require.NoError(t, err)
				appServer.SetOrigin(types.OriginOkta)
				_, err = a.UpsertApplicationServer(context.Background(), appServer)
				require.NoError(t, err)
			},
			deleter: func(t *testing.T, hostID string) {
				require.NoError(t, a.DeleteApplicationServer(t.Context(), defaults.Namespace, hostID, "test-okta-app"))
			},
		},
	}

	a.SetEC2ClientForEC2JoinMethod(ec2ClientRunning{})
	nopClient, err := testServer.NewClient(authtest.TestNop())
	require.NoError(t, err)
	a.SetClock(clockwork.NewFakeClockAt(instance1.pendingTime))

	for _, tc := range testCases {
		t.Run(string(tc.role), func(t *testing.T) {
			// Join works with no existing heartbeat.
			_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
				Token: "test_token",
				ID: state.IdentityID{
					Role:     types.RoleInstance,
					NodeName: "testnode",
				},
				AuthClient: nopClient,
				GetInstanceIdentityDocumentFunc: func(_ context.Context) ([]byte, error) {
					return instance1.iid, nil
				},
			})
			require.NoError(t, err)

			if tc.upserter != nil {
				// Add the resource heartbeat.
				name := instance1.account + "-" + instance1.instanceID
				tc.upserter(t, name)

				// Delete the resource heartbeat at the end of the test.
				if tc.deleter != nil {
					t.Cleanup(func() {
						tc.deleter(t, name)
					})
				}

				// Join should fail.
				t.Run("new", func(t *testing.T) {
					_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
						Token: "test_token",
						ID: state.IdentityID{
							Role:     types.RoleInstance,
							NodeName: "testnode",
						},
						AuthClient: nopClient,
						GetInstanceIdentityDocumentFunc: func(_ context.Context) ([]byte, error) {
							return instance1.iid, nil
						},
					})
					require.ErrorAs(t, err, new(*trace.AccessDeniedError), err)
					require.ErrorContains(t, err, "already exists")
				})
				t.Run("legacy", func(t *testing.T) {
					_, err := joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
						Token:      "test_token",
						JoinMethod: types.JoinMethodEC2,
						ID: state.IdentityID{
							Role:     tc.role,
							NodeName: "testnode",
							HostUUID: instance1.account + "-" + instance1.instanceID,
						},
						AuthClient: nopClient,
						GetInstanceIdentityDocumentFunc: func(_ context.Context) ([]byte, error) {
							return instance1.iid, nil
						},
					})
					require.ErrorAs(t, err, new(*trace.AccessDeniedError), err)
					require.ErrorContains(t, err, "already exists")
				})
			}
		})
	}
}
