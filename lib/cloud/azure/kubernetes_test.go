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

package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
)

const (
	clusterName = "cluster1"
	groupName   = "group1"
	tenantID    = "tenant1"
	region      = "west-1"
	subID       = "subid1"
)

func Test_aKSClient_ClusterCredentials(t *testing.T) {
	type fields struct {
		api ARMAKS
	}
	type args struct {
		cfg ClusterCredentialsConfig
	}
	tests := []struct {
		name               string
		fields             fields
		args               args
		validateRestConfig func(*testing.T, *rest.Config)
		wantErr            bool
	}{
		{
			name: "local accounts",
			fields: fields{
				api: &ARMKubernetesMock{
					KubeServers: []*armcontainerservice.ManagedCluster{
						aksClusterToManagedCluster(
							AKSCluster{
								Name:           clusterName,
								GroupName:      groupName,
								TenantID:       tenantID,
								Location:       region,
								SubscriptionID: subID,
								Properties: AKSClusterProperties{
									AccessConfig:  LocalAccounts,
									LocalAccounts: true,
								},
							},
						),
					},
					ClusterUserCreds: kubeConfigToBin(clusterName, false),
				},
			},
			args: args{
				cfg: ClusterCredentialsConfig{
					ResourceName:  clusterName,
					ResourceGroup: groupName,
					TenantID:      tenantID,
					ImpersonationPermissionsChecker: func(ctx context.Context, clusterName string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
						return nil
					},
				},
			},
			validateRestConfig: func(t *testing.T, c *rest.Config) {
				require.Equal(t, c.Username, "exp")
			},
		},
		{
			name: "azure RBAC accounts",
			fields: fields{

				api: &ARMKubernetesMock{
					KubeServers: []*armcontainerservice.ManagedCluster{
						aksClusterToManagedCluster(
							AKSCluster{
								Name:           clusterName,
								GroupName:      groupName,
								TenantID:       tenantID,
								Location:       region,
								SubscriptionID: subID,
								Properties: AKSClusterProperties{
									AccessConfig:  AzureRBAC,
									LocalAccounts: false,
								},
							},
						),
					},
					ClusterUserCreds: kubeConfigToBin(clusterName, true),
				},
			},
			args: args{
				cfg: ClusterCredentialsConfig{
					ResourceName:  clusterName,
					ResourceGroup: groupName,
					TenantID:      tenantID,
					ImpersonationPermissionsChecker: func(ctx context.Context, clusterName string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
						return nil
					},
				},
			},
			validateRestConfig: func(t *testing.T, c *rest.Config) {
				require.NotEmpty(t, c.BearerToken)
				require.Nil(t, c.ExecProvider)
			},
		},
		{
			name: "azure AD accounts",
			fields: fields{

				api: &ARMKubernetesMock{
					KubeServers: []*armcontainerservice.ManagedCluster{
						aksClusterToManagedCluster(
							AKSCluster{
								Name:           clusterName,
								GroupName:      groupName,
								TenantID:       tenantID,
								Location:       region,
								SubscriptionID: subID,
								Properties: AKSClusterProperties{
									AccessConfig:  AzureAD,
									LocalAccounts: false,
								},
							},
						),
					},
					ClusterUserCreds:  kubeConfigToBin(clusterName, true),
					ClusterAdminCreds: kubeConfigToBin(clusterName, false),
				},
			},
			args: args{
				cfg: ClusterCredentialsConfig{
					ResourceName:  clusterName,
					ResourceGroup: groupName,
					TenantID:      tenantID,
					ImpersonationPermissionsChecker: func(ctx context.Context, clusterName string, sarClient authztypes.SelfSubjectAccessReviewInterface) error {
						return nil
					},
				},
			},
			validateRestConfig: func(t *testing.T, c *rest.Config) {
				require.NotEmpty(t, c.BearerToken)
				require.Nil(t, c.ExecProvider)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			azIdentity := &azIdentityMock{t: t}
			c := NewAKSClustersClient(tt.fields.api, func(options *azidentity.DefaultAzureCredentialOptions) (GetToken, error) {
				return azIdentity, nil
			})
			got, _, err := c.ClusterCredentials(context.TODO(), tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("aKSClient.ClusterCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			tt.validateRestConfig(t, got)

		})
	}
}

func aksClusterToManagedCluster(cluster AKSCluster) *armcontainerservice.ManagedCluster {
	id := fmt.Sprintf("/subscriptions/%s/resourcegroups/%s/providers/Microsoft.ContainerService/managedClusters/%s", cluster.SubscriptionID, cluster.GroupName, cluster.Name)
	aCluster := &armcontainerservice.ManagedCluster{
		Name:     to.Ptr(cluster.Name),
		Location: to.Ptr(cluster.Location),
		Tags:     convertTagsToAKS(cluster.Tags),
		Identity: &armcontainerservice.ManagedClusterIdentity{
			TenantID: to.Ptr(cluster.TenantID),
		},
		Properties: &armcontainerservice.ManagedClusterProperties{
			ProvisioningState: to.Ptr("Succeeded"),
			PowerState: &armcontainerservice.PowerState{
				Code: to.Ptr(armcontainerservice.CodeRunning),
			},
		},
		ID: to.Ptr(id),
	}

	switch cluster.Properties.AccessConfig {
	case AzureRBAC:
		aCluster.Properties.AADProfile = &armcontainerservice.ManagedClusterAADProfile{
			EnableAzureRBAC: to.Ptr(true),
		}
	case AzureAD:
		aCluster.Properties.AADProfile = &armcontainerservice.ManagedClusterAADProfile{}
		if !cluster.Properties.LocalAccounts {
			aCluster.Properties.DisableLocalAccounts = to.Ptr(true)
		}
	case LocalAccounts:

	}
	return aCluster
}

func convertTagsToAKS(t map[string]string) map[string]*string {
	tags := make(map[string]*string)
	for k, v := range t {
		tags[k] = to.Ptr(v)
	}
	return tags
}

func kubeConfigToBin(clusterName string, isExec bool) *armcontainerservice.CredentialResult {
	if !isExec {
		return &armcontainerservice.CredentialResult{
			Name: nil,
			Value: []byte(`apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://5.6.7.8
  name: scratch
contexts:
- context:
    cluster: scratch
    user: experimenter
  name: experimenter
current-context: "experimenter"
kind: Config
users:
- name: experimenter
  user:
    password: some-password
    username: exp`),
		}
	}
	kubeConfig := `
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://5.6.7.8
  name: scratch
contexts:
- context:
    cluster: scratch
    user: experimenter
  name: experimenter
current-context: "experimenter"
kind: Config
users:
- name: tele.teleport.local-tiago-test
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: tsh`

	return &armcontainerservice.CredentialResult{
		Name:  nil,
		Value: []byte(kubeConfig),
	}
}

type azIdentityMock struct {
	t *testing.T
}

func (a *azIdentityMock) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token: newToken(a.t),
	}, nil
}

func newToken(t *testing.T) string {

	str, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &azureGroupClaims{
		Groups: []string{"groupID"},
	}).SignedString([]byte("test"))

	require.NoError(t, err)
	return str
}
