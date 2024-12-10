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

package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gravitational/trace"
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

func Test_AKSClient_ClusterCredentials(t *testing.T) {
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
		groupID            string
		validateRestConfig func(*testing.T, *rest.Config)
		checkErr           require.ErrorAssertionFunc
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
			groupID:  "groupID",
			checkErr: require.NoError,
			validateRestConfig: func(t *testing.T, c *rest.Config) {
				require.Equal(t, "exp", c.Username)
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
			groupID:  "groupID",
			checkErr: require.NoError,
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
			groupID:  "groupID",
			checkErr: require.NoError,
			validateRestConfig: func(t *testing.T, c *rest.Config) {
				require.NotEmpty(t, c.BearerToken)
				require.Nil(t, c.ExecProvider)
			},
		},
		{
			name: "azure AD accounts no claims",
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
						return trace.AccessDenied("access denied")
					},
				},
			},
			groupID:  "",
			checkErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			azIdentity := &azIdentityMock{t: t, groupID: tt.groupID}
			c := NewAKSClustersClient(tt.fields.api, func(options *azidentity.DefaultAzureCredentialOptions) (GetToken, error) {
				return azIdentity, nil
			})
			got, _, err := c.ClusterCredentials(context.TODO(), tt.args.cfg)
			tt.checkErr(t, err)

			if tt.validateRestConfig != nil {
				tt.validateRestConfig(t, got)
			}
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
	t       *testing.T
	groupID string
}

func (a *azIdentityMock) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token: newToken(a.t, a.groupID),
	}, nil
}

func newToken(t *testing.T, groupID string) string {
	claims := &azureGroupClaims{}
	if groupID != "" {
		claims.Groups = []string{groupID}
	}
	str, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test"))
	require.NoError(t, err)
	return str
}
