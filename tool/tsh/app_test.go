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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/client"
)

func TestFormatAppConfig(t *testing.T) {
	t.Parallel()

	defaultTc := &client.TeleportClient{
		Config: client.Config{
			WebProxyAddr: "test-tp.teleport:8443",
		},
	}
	testProfile := &client.ProfileStatus{
		Username: "test-user",
		Dir:      "/test/dir",
	}
	testAppName := "test-tp"
	testAppPublicAddr := "test-tp.teleport"
	testCluster := "test-tp"

	// func formatAppConfig(tc *client.TeleportClient, profile *client.ProfileStatus, appName,
	// appPublicAddr, format, cluster string) (string, error) {
	tests := []struct {
		name              string
		tc                *client.TeleportClient
		format            string
		awsArn            string
		azureIdentity     string
		gcpServiceAccount string
		insecure          bool
		expected          string
		wantErr           bool
	}{
		{
			name: "format URI standard HTTPS port",
			tc: &client.TeleportClient{
				Config: client.Config{
					WebProxyAddr: "test-tp.teleport:443",
				},
			},
			format:   appFormatURI,
			expected: "https://test-tp.teleport",
		},
		{
			name:     "format URI standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatURI,
			expected: "https://test-tp.teleport:8443",
		},
		{
			name:     "format CA",
			tc:       defaultTc,
			format:   appFormatCA,
			expected: "/test/dir/keys/cas/test-tp.pem",
		},
		{
			name:     "format cert",
			tc:       defaultTc,
			format:   appFormatCert,
			expected: "/test/dir/keys/test-user-app/test-tp-x509.pem",
		},
		{
			name:     "format key",
			tc:       defaultTc,
			format:   appFormatKey,
			expected: "/test/dir/keys/test-user",
		},
		{
			name:   "format curl standard non-standard HTTPS port",
			tc:     defaultTc,
			format: appFormatCURL,
			expected: `curl \
  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
  --key /test/dir/keys/test-user \
  https://test-tp.teleport:8443`,
		},
		{
			name:     "format insecure curl standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatCURL,
			insecure: true,
			expected: `curl --insecure \
  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
  --key /test/dir/keys/test-user \
  https://test-tp.teleport:8443`,
		},
		{
			name:   "format JSON",
			tc:     defaultTc,
			format: appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443"
}
`,
		},
		{
			name:   "format YAML",
			tc:     defaultTc,
			format: appFormatYAML,
			expected: `ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
		{
			name:   "format default",
			tc:     defaultTc,
			format: "default",
			expected: `Name:      test-tp                                       
URI:       https://test-tp.teleport:8443                 
CA:        /test/dir/keys/cas/test-tp.pem                
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:       /test/dir/keys/test-user                      
`,
		},
		{
			name:   "empty format means default",
			tc:     defaultTc,
			format: "",
			expected: `Name:      test-tp                                       
URI:       https://test-tp.teleport:8443                 
CA:        /test/dir/keys/cas/test-tp.pem                
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:       /test/dir/keys/test-user                      
`,
		},
		{
			name:    "reject invalid format",
			tc:      defaultTc,
			format:  "invalid",
			wantErr: true,
		},
		// Azure
		{
			name:          "azure default format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        "default",
			expected: `Name:      test-tp                                                                                                                                                        
URI:       https://test-tp.teleport:8443                                                                                                                                  
CA:        /test/dir/keys/cas/test-tp.pem                                                                                                                                 
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem                                                                                                                  
Key:       /test/dir/keys/test-user                                                                                                                                       
Azure Id:  /subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure 
`,
		},
		{
			name:          "azure JSON format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443",
  "azure_identity": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"
}
`,
		},
		{
			name:          "azure YAML format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        appFormatYAML,
			expected: `azure_identity: /subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure
ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
		// GCP
		{
			name:              "gcp default format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            "default",
			expected: `Name:                test-tp                                       
URI:                 https://test-tp.teleport:8443                 
CA:                  /test/dir/keys/cas/test-tp.pem                
Cert:                /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:                 /test/dir/keys/test-user                      
GCP Service Account: dev@example-123456.iam.gserviceaccount.com    
`,
		},
		{
			name:              "gcp JSON format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443",
  "gcp_service_account": "dev@example-123456.iam.gserviceaccount.com"
}
`,
		},
		{
			name:              "gcp YAML format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatYAML,
			expected: `ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
gcp_service_account: dev@example-123456.iam.gserviceaccount.com
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.tc.InsecureSkipVerify = test.insecure
			result, err := formatAppConfig(test.tc, testProfile, testAppName, testAppPublicAddr, test.format, testCluster, test.awsArn, test.azureIdentity, test.gcpServiceAccount)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}
