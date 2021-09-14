package auth

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	check "gopkg.in/check.v1"
)

type ec2Instance struct {
	iid        []byte
	account    string
	region     string
	instanceID string
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
		account:    "278576220453",
		region:     "us-west-2",
		instanceID: "i-078517ca8a70a1dde",
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
		account:    "883474662888",
		region:     "us-west-1",
		instanceID: "i-01b940c45fd11fe74",
	}
)

type ec2ClientNoInstance struct{}
type ec2ClientNotRunning struct{}
type ec2ClientRunning struct{}

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

func (s *AuthSuite) TestSimplifiedNodeJoin(c *check.C) {
	err := s.a.UpsertNamespace(types.DefaultNamespace())
	c.Assert(err, check.IsNil)

	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      instance2.account + "-" + instance2.instanceID,
			Namespace: apidefaults.Namespace,
		},
		Spec: types.ServerSpecV2{
			Addr: "localhost:3022",
		},
	}
	_, err = s.a.UpsertNode(context.Background(), node)
	c.Assert(err, check.IsNil)

	testCases := []struct {
		desc       string
		tokenRules []*types.TokenRule
		ec2Client  ec2Client
		request    RegisterUsingTokenRequest
		expected   check.Checker
	}{
		{
			desc: "basic passing case",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance1.account,
					AWSRegions: []string{instance1.region},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expected: check.IsNil,
		},
		{
			desc: "pass with multiple rules",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance2.account,
					AWSRegions: []string{instance2.region},
				},
				&types.TokenRule{
					AWSAccount: instance1.account,
					AWSRegions: []string{instance1.region},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expected: check.IsNil,
		},
		{
			desc: "wrong account",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: "bad account",
					AWSRegions: []string{instance1.region},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expected: check.NotNil,
		},
		{
			desc: "wrong region",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance1.account,
					AWSRegions: []string{"bad region"},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expected: check.NotNil,
		},
		{
			desc: "bad HostID",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance1.account,
					AWSRegions: []string{instance1.region},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              "bad host id",
				EC2IdentityDocument: instance1.iid,
			},
			expected: check.NotNil,
		},
		{
			desc: "bad identity document",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance1.account,
					AWSRegions: []string{instance1.region},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: []byte("bad document"),
			},
			expected: check.NotNil,
		},
		{
			desc: "instance already joined",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance2.account,
					AWSRegions: []string{instance2.region},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance2.account + "-" + instance2.instanceID,
				EC2IdentityDocument: instance2.iid,
			},
			expected: check.NotNil,
		},
		{
			desc: "instance already joined, fake ID",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance2.account,
					AWSRegions: []string{instance2.region},
				},
			},
			ec2Client: ec2ClientRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              "fake id",
				EC2IdentityDocument: instance2.iid,
			},
			expected: check.NotNil,
		},
		{
			desc: "instance not running",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance1.account,
					AWSRegions: []string{instance1.region},
				},
			},
			ec2Client: ec2ClientNotRunning{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expected: check.NotNil,
		},
		{
			desc: "instance not exists",
			tokenRules: []*types.TokenRule{
				&types.TokenRule{
					AWSAccount: instance1.account,
					AWSRegions: []string{instance1.region},
				},
			},
			ec2Client: ec2ClientNoInstance{},
			request: RegisterUsingTokenRequest{
				Token:               "test_token",
				NodeName:            "node_name",
				Role:                types.RoleNode,
				HostID:              instance1.account + "-" + instance1.instanceID,
				EC2IdentityDocument: instance1.iid,
			},
			expected: check.NotNil,
		},
	}
	for _, tc := range testCases {
		println("Running test case:", tc.desc)
		token, err := types.NewProvisionTokenFromSpec(
			"test_token",
			time.Now().Add(time.Minute),
			types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: tc.tokenRules,
			})
		c.Assert(err, check.IsNil)

		err = s.a.UpsertToken(context.Background(), token)
		c.Assert(err, check.IsNil)

		ctx := context.WithValue(context.Background(), ec2ClientKey{}, tc.ec2Client)

		err = s.a.CheckEC2Request(ctx, tc.request)
		c.Assert(err, tc.expected)

		err = s.a.DeleteToken(context.Background(), token.GetName())
		c.Assert(err, check.IsNil)
	}
}
