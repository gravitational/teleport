// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
package tbot

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spiffe/aws-spiffe-workload-helper/vendoredaws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func Test_renderAWSCreds(t *testing.T) {
	creds := &vendoredaws.CredentialProcessOutput{
		AccessKeyId:     "AKIAIOSFODNN7EXAMPLEAKID",
		SessionToken:    "AQoDYXdzEJrtyWJ4NjK7PiEXAMPLEST",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLESAK",
	}
	ctx := context.Background()

	dest := &config.DestinationMemory{}
	require.NoError(t, dest.CheckAndSetDefaults())
	require.NoError(t, dest.Init(ctx, []string{}))

	err := renderAWSCreds(ctx, creds, dest)
	require.NoError(t, err)

	got, err := dest.Read(ctx, "aws_credentials")
	require.NoError(t, err)

	if golden.ShouldSet() {
		golden.Set(t, got)
	}
	require.Equal(t, golden.Get(t), got)
}

type mockCreateSessionInputBody struct {
	DurationSeconds int `json:"durationSeconds"`
}

func TestBotWorkloadIdentityAWSRA(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	process := testenv.MakeTestServer(t, defaultTestServerOpts(t, log))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	roleArn := "arn:aws:iam::123456789012:role/example-role"
	trustAnchorArn := "arn:aws:rolesanywhere:us-east-1:123456789012:trust-anchor/0000000-0000-0000-0000-000000000000"
	profileArn := "arn:aws:rolesanywhere:us-east-1:123456789012:profile/0000000-0000-0000-0000-00000000000"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/sessions", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Check query parameter inputs
		// The AWS documentation "lies" about these inputs using the JSON body
		// - the rolesanywhere API client in
		//  `aws/rolesanywhere-credential-helper` uses query parameters for
		// these.
		assert.Equal(t, roleArn, r.URL.Query().Get("roleArn"))
		assert.Equal(t, trustAnchorArn, r.URL.Query().Get("trustAnchorArn"))
		assert.Equal(t, profileArn, r.URL.Query().Get("profileArn"))

		// Check JSON body inputs
		body := &mockCreateSessionInputBody{}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(body))
		assert.Equal(t, int((2 * time.Hour).Seconds()), body.DurationSeconds)

		// Validate the X-Amz-X509 header contains the valid (and correct) SVID
		derString := r.Header.Get("X-Amz-X509")
		assert.NotEmpty(t, derString)
		derBytes, err := base64.StdEncoding.DecodeString(derString)
		assert.NoError(t, err)
		cert, err := x509.ParseCertificate(derBytes)
		assert.NoError(t, err)
		assert.Len(t, cert.URIs, 1)
		assert.Equal(t, "spiffe://root/ra-test", cert.URIs[0].String())

		// Validate the authorization header exists. We rely on the AWS SDK to
		// actually produce the signature, and, validating this signature would
		// introduce significant complexity to this test - so this is omitted.
		authz := r.Header.Get("Authorization")
		assert.NotEmpty(t, authz)

		// Send mocked response
		_, _ = w.Write([]byte(`{
			"credentialSet":[
			  {
				"assumedRoleUser": {
				"arn": "arn:aws:iam::123456789012:role/example-role",
				"assumedRoleId": "assumedRoleId"
				},
				"credentials":{
				  "accessKeyId": "accessKeyId",
				  "expiration": "2028-07-27T04:36:55Z",
				  "secretAccessKey": "secretAccessKey",
				  "sessionToken": "sessionToken"
				},
				"packedPolicySize": 10,
				"roleArn": "arn:aws:iam::123456789012:role/example-role",
				"sourceIdentity": "sourceIdentity"
			  }
			],
			"subjectArn": "arn:aws:rolesanywhere:us-east-1:000000000000:subject/41cl0bae-6783-40d4-ab20-65dc5d922e45"
		  }`))
	}))
	t.Cleanup(srv.Close)

	role, err := types.NewRole("issue-foo", types.RoleSpecV6{
		Allow: types.RoleConditions{
			WorkloadIdentityLabels: map[string]apiutils.Strings{
				"foo": []string{"bar"},
			},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindWorkloadIdentity},
					Verbs:     []string{types.VerbRead, types.VerbList},
				},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	workloadIdentity := &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "foo-bar-bizz",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/ra-test",
			},
		},
	}
	workloadIdentity, err = rootClient.WorkloadIdentityResourceServiceClient().
		CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: workloadIdentity,
		})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	onboarding, _ := makeBot(t, rootClient, "ra-test", role.GetName())
	botConfig := defaultBotConfig(t, process, onboarding, config.ServiceConfigs{
		&config.WorkloadIdentityAWSRAService{
			Selector: config.WorkloadIdentitySelector{
				Name: workloadIdentity.GetMetadata().GetName(),
			},
			Destination: &config.DestinationDirectory{
				Path: tmpDir,
			},
			RoleARN:                roleArn,
			ProfileARN:             profileArn,
			TrustAnchorARN:         trustAnchorArn,
			Region:                 "us-east-1",
			SessionDuration:        2 * time.Hour,
			SessionRenewalInterval: 30 * time.Minute,
			EndpointOverride:       srv.URL,
		},
	}, defaultBotConfigOpts{
		useAuthServer: true,
		insecure:      true,
	})

	botConfig.Oneshot = true
	b := New(botConfig, log)
	// Run Bot with 10 second timeout to catch hangs.
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	t.Cleanup(cancel)
	require.NoError(t, b.Run(ctx))

	got, err := os.ReadFile(filepath.Join(tmpDir, "aws_credentials"))
	require.NoError(t, err)
	if golden.ShouldSet() {
		golden.Set(t, got)
	}
	require.Equal(t, string(golden.Get(t)), string(got))
}
