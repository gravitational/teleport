/*
Copyright 2022 Gravitational, Inc.

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

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestAWSExecCommand tests launching a command under tsh AWS proxy.
//
// "curl" is run under the tsh proxy to send a request to a local test server,
// and the test verifies the request is forwarded by our proxy then the local
// test server receives it. Note that AWS requests are not tested to avoid app
// server sends out test requests to AWS.
func TestAWSExecCommand(t *testing.T) {
	_, err := exec.LookPath("curl")
	if err != nil {
		t.Skip("Skipping. No external curl binary found.")
		return
	}

	// Setup suite.
	awsAppName := "aws-app"
	awsRoleARN := "arn:aws:iam::1234567890:role/test-role"
	awsRole, err := types.NewRoleV3("aws-acess", types.RoleSpecV5{
		Allow: types.RoleConditions{
			AWSRoleARNs: []string{awsRoleARN},
		},
	})
	require.NoError(t, err)

	user, err := types.NewUser("bob")
	require.NoError(t, err)
	user.SetRoles([]string{awsRole.GetName()})

	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *service.Config) {
			cfg.Auth.Resources = append(cfg.Auth.Resources, user, awsRole)
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = append(cfg.Apps.Apps, service.App{
				Name: awsAppName,
				URI:  "https://console.aws.amazon.com/ec2/v2/home",
			})
		}),
	)

	// Login user.
	t.Setenv("TELEPORT_HOME", t.TempDir())
	require.NoError(t, Run([]string{
		"login",
		"--insecure",
		"--user", user.GetName(),
		"--auth", s.connector.GetName(),
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), user)
		return nil
	}))

	// Login app. Retry a few times while apps are being registered.
	require.NoError(t, utils.RetryStaticFor(time.Second*10, time.Millisecond*500, func() error {
		err := Run([]string{
			"apps",
			"login",
			"--aws-role", awsRoleARN,
			"--insecure",
			awsAppName,
		})
		return trace.Wrap(err)
	}))

	// Run curl against test server to avoid sending real AWS APIs out.
	var stdout bytes.Buffer
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello test"))
	}))
	require.NoError(t, Run([]string{
		"aws",
		"-d",
		"--exec",
		"-v",
		"curl",
		"-k",
		testServer.URL,
	}, func(cf *CLIConf) error {
		cf.overrideStdout = &stdout
		return nil
	}))
	require.Equal(t, "hello test", stdout.String())
}
