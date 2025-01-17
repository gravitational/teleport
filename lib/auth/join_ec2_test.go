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

package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
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

func TestAuth_RegisterUsingToken_EC2(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	// upsert a node to test duplicates
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      instance2.account + "-" + instance2.instanceID,
			Namespace: defaults.Namespace,
		},
	}
	_, err = a.UpsertNode(ctx, node)
	require.NoError(t, err)

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	isNil := func(err error) bool {
		return err == nil
	}

	testCases := []struct {
		desc        string
		tokenSpec   types.ProvisionTokenSpecV2
		ec2Client   ec2Client
		request     types.RegisterUsingTokenRequest
		expectError func(error) bool
		clock       clockwork.Clock
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: isNil,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: isNil,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: isNil,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: isNil,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              "bad host id",
				EC2IdentityDocument: instance1.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:    "test_token",
				NodeName: "node_name",
				Role:     types.RoleNode,
				HostID:   instance1.account + "-" + instance1.instanceID,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: []byte("bad document"),
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance2.account + "-" + instance2.instanceID,
				EC2IdentityDocument: instance2.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance2.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              "fake id",
				EC2IdentityDocument: instance2.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance2.pendingTime),
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
			ec2Client: ec2ClientNotRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientNoInstance{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime.Add(5*time.Minute + time.Second)),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: isNil,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime.Add(9 * time.Minute)),
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
			ec2Client: ec2ClientRunning{},
			request: types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expectError: trace.IsAccessDenied,
			clock:       clockwork.NewFakeClockAt(instance1.pendingTime.Add(11 * time.Minute)),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			clock := tc.clock
			if clock == nil {
				clock = clockwork.NewRealClock()
			}
			a.clock = clock

			token, err := types.NewProvisionTokenFromSpec(
				"test_token",
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)

			err = a.UpsertToken(context.Background(), token)
			require.NoError(t, err)

			ctx := context.WithValue(context.Background(), ec2ClientKey{}, tc.ec2Client)

			// set common request values here to avoid setting them in every
			// testcase
			tc.request.PublicSSHKey = sshPublicKey
			tc.request.PublicTLSKey = tlsPublicKey

			_, err = a.RegisterUsingToken(ctx, &tc.request)
			require.True(t, tc.expectError(err))

			err = a.DeleteToken(context.Background(), token.GetName())
			require.NoError(t, err)
		})
	}
}

// TestAWSCerts asserts that all certificates parse
func TestAWSCerts(t *testing.T) {
	for _, certBytes := range awsRSA2048CertBytes {
		certPEM, _ := pem.Decode(certBytes)
		_, err := x509.ParseCertificate(certPEM.Bytes)
		require.NoError(t, err)
	}
}

// TestHostUniqueCheck tests the uniqueness check used by checkEC2JoinRequest
func TestHostUniqueCheck(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	a.clock = clockwork.NewFakeClockAt(instance1.pendingTime)

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

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	testCases := []struct {
		role     types.SystemRole
		upserter func(name string)
	}{
		{
			role: types.RoleNode,
			upserter: func(name string) {
				node := &types.ServerV2{
					Kind:    types.KindNode,
					Version: types.V2,
					Metadata: types.Metadata{
						Name:      name,
						Namespace: defaults.Namespace,
					},
				}
				_, err := a.UpsertNode(context.Background(), node)
				require.NoError(t, err)
			},
		},
		{
			role: types.RoleProxy,
			upserter: func(name string) {
				proxy := &types.ServerV2{
					Kind:    types.KindProxy,
					Version: types.V2,
					Metadata: types.Metadata{
						Name:      name,
						Namespace: defaults.Namespace,
					},
				}
				err := a.UpsertProxy(context.Background(), proxy)
				require.NoError(t, err)
			},
		},
		{
			role: types.RoleKube,
			upserter: func(name string) {
				kube, err := types.NewKubernetesServerV3(
					types.Metadata{
						Name:      name,
						Namespace: defaults.Namespace,
					},
					types.KubernetesServerSpecV3{
						HostID:   name,
						Hostname: "test-kuge",
						Cluster: &types.KubernetesClusterV3{
							Metadata: types.Metadata{
								Name:      name,
								Namespace: defaults.Namespace,
							},
						},
					})
				require.NoError(t, err)
				_, err = a.UpsertKubernetesServer(context.Background(), kube)
				require.NoError(t, err)
			},
		},
		{
			role: types.RoleDatabase,
			upserter: func(name string) {
				db, err := types.NewDatabaseServerV3(
					types.Metadata{
						Name:      name,
						Namespace: defaults.Namespace,
					},
					types.DatabaseServerSpecV3{
						HostID:   name,
						Hostname: "test-db",
						Database: &types.DatabaseV3{
							Metadata: types.Metadata{
								Name:      "test-db",
								Namespace: defaults.Namespace,
							},
							Spec: types.DatabaseSpecV3{
								Protocol: types.DatabaseProtocolPostgreSQL,
								URI:      "https://db.localhost",
							},
						},
					})
				require.NoError(t, err)
				_, err = a.UpsertDatabaseServer(context.Background(), db)
				require.NoError(t, err)
			},
		},
		{
			role: types.RoleApp,
			upserter: func(name string) {
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
						Name:      name,
						Namespace: defaults.Namespace,
					},
					types.AppServerSpecV3{
						HostID: name,
						App:    app,
					})
				require.NoError(t, err)
				_, err = a.UpsertApplicationServer(context.Background(), appServer)
				require.NoError(t, err)
			},
		},
		{
			role: types.RoleWindowsDesktop,
			upserter: func(name string) {
				wds, err := types.NewWindowsDesktopServiceV3(types.Metadata{Name: instance1.account + "-" + instance1.instanceID},
					types.WindowsDesktopServiceSpecV3{
						Addr:            "localhost:3028",
						TeleportVersion: "10.2.2",
					})
				require.NoError(t, err)

				_, err = a.UpsertWindowsDesktopService(context.Background(), wds)
				require.NoError(t, err)
			},
		},
		{
			role: types.RoleOkta,
			upserter: func(name string) {
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
						Name:      name,
						Namespace: defaults.Namespace,
					},
					types.AppServerSpecV3{
						HostID: name,
						App:    app,
					})
				require.NoError(t, err)
				appServer.SetOrigin(types.OriginOkta)
				_, err = a.UpsertApplicationServer(context.Background(), appServer)
				require.NoError(t, err)
			},
		},
		{
			role: types.RoleDiscovery,
		},
		{
			role: types.RoleMDM,
		},
	}

	ctx = context.WithValue(ctx, ec2ClientKey{}, ec2ClientRunning{})

	for _, tc := range testCases {
		t.Run(string(tc.role), func(t *testing.T) {
			request := types.RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                tc.role,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
				PublicSSHKey:        sshPublicKey,
				PublicTLSKey:        tlsPublicKey,
			}

			// request works with no existing host
			_, err = a.RegisterUsingToken(ctx, &request)
			require.NoError(t, err)

			if tc.upserter != nil {
				// add the server
				name := instance1.account + "-" + instance1.instanceID
				tc.upserter(name)

				// request should fail
				_, err = a.RegisterUsingToken(ctx, &request)
				expectedErr := &trace.AccessDeniedError{}
				require.ErrorAs(t, err, &expectedErr, err)
			}
		})
	}
}
