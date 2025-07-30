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
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	armazcore "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/rbac/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/lib/fixtures"
)

// AKSAuthMethod defines the authentication method for AKS cluster.
type AKSAuthMethod uint8

const (
	// AzureRBAC indicates that the Azure AD is enabled and authorization is handled by Azure RBAC.
	AzureRBAC AKSAuthMethod = iota
	// AzureAD indicates that the Azure AD is enabled but authorization is handled by Kubernetes RBAC.
	AzureAD
	// LocalAccounts indicates that the cluster access happens through Local accounts created
	// during provisioning phase.
	LocalAccounts
)

// AKSCluster represents an AKS cluster.
type AKSCluster struct {
	// Name is the name of the cluster.
	Name string
	// GroupName is the resource group name.
	GroupName string
	// TenantID is the cluster TenantID.
	TenantID string
	// Location is the cluster region.
	Location string
	// SubscriptionID is the cluster subscription id.
	SubscriptionID string
	// Tags are the cluster tags.
	Tags map[string]string
	// Properties are the cluster authentication and authorization properties.
	Properties AKSClusterProperties
}

// AKSClusterProperties holds the AZ cluster authentication properties.
type AKSClusterProperties struct {
	// AccessConfig indicates the authentication & authorization config to use with the cluster.
	AccessConfig AKSAuthMethod
	// LocalAccounts indicates if the cluster has local accounts.
	LocalAccounts bool
}

// ARMAKS is an interface for armcontainerservice.ManagedClustersClient.
type ARMAKS interface {
	BeginRunCommand(ctx context.Context, resourceGroupName string, resourceName string, requestPayload armcontainerservice.RunCommandRequest, options *armcontainerservice.ManagedClustersClientBeginRunCommandOptions) (*runtime.Poller[armcontainerservice.ManagedClustersClientRunCommandResponse], error)
	Get(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientGetOptions) (armcontainerservice.ManagedClustersClientGetResponse, error)
	GetCommandResult(ctx context.Context, resourceGroupName string, resourceName string, commandID string, options *armcontainerservice.ManagedClustersClientGetCommandResultOptions) (armcontainerservice.ManagedClustersClientGetCommandResultResponse, error)
	ListClusterAdminCredentials(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientListClusterAdminCredentialsOptions) (armcontainerservice.ManagedClustersClientListClusterAdminCredentialsResponse, error)
	ListClusterUserCredentials(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientListClusterUserCredentialsOptions) (armcontainerservice.ManagedClustersClientListClusterUserCredentialsResponse, error)
	NewListByResourceGroupPager(resourceGroupName string, options *armcontainerservice.ManagedClustersClientListByResourceGroupOptions) *runtime.Pager[armcontainerservice.ManagedClustersClientListByResourceGroupResponse]
	NewListPager(options *armcontainerservice.ManagedClustersClientListOptions) *runtime.Pager[armcontainerservice.ManagedClustersClientListResponse]
}

var _ ARMAKS = (*armcontainerservice.ManagedClustersClient)(nil)

// ImpersonationPermissionsChecker describes a function that can be used to check
// for the required impersonation permissions on a Kubernetes cluster. Return nil
// to indicate success.
type ImpersonationPermissionsChecker func(ctx context.Context, clusterName string,
	sarClient authztypes.SelfSubjectAccessReviewInterface) error

// azureIdentityFunction is a function signature used to setup azure credentials.
// This is used to generate special credentials with cluster TentantID to retrieve
// access tokens.
type azureIdentityFunction func(options *azidentity.DefaultAzureCredentialOptions) (GetToken, error)

// GetToken is an interface for generating tokens from credentials.
type GetToken interface {
	// GetToken returns an azure token.
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// ClusterCredentialsConfig are the required parameters for generating cluster credentials.
type ClusterCredentialsConfig struct {
	// ResourceGroup is the AKS cluster resource group.
	ResourceGroup string
	// ResourceName is the AKS cluster name.
	ResourceName string
	// TenantID is the AKS cluster tenant id.
	TenantID string
	// ImpersonationPermissionsChecker is checker function that validates if access
	// was granted.
	ImpersonationPermissionsChecker ImpersonationPermissionsChecker
}

// CheckAndSetDefaults checks for required parameters.
func (c ClusterCredentialsConfig) CheckAndSetDefaults() error {
	if len(c.ResourceGroup) == 0 {
		return trace.BadParameter("invalid ResourceGroup field")
	}
	if len(c.ResourceName) == 0 {
		return trace.BadParameter("invalid ResourceName field")
	}
	if c.ImpersonationPermissionsChecker == nil {
		return trace.BadParameter("invalid ImpersonationPermissionsChecker field")
	}
	return nil
}

// AKSClient is the Azure client to interact with AKS.
type AKSClient interface {
	// ListAll returns all AKSClusters the user has access to.
	ListAll(ctx context.Context) ([]*AKSCluster, error)
	// ListAll returns all AKSClusters the user has access to within the resource group.
	ListWithinGroup(ctx context.Context, group string) ([]*AKSCluster, error)
	// ClusterCredentials returns the credentials for accessing the desired AKS cluster.
	// If agent access has not yet been configured, this function will attempt to configure it
	// using administrator credentials `ListClusterAdminCredentials`` or by running a command `BeginRunCommand`.
	// If the access setup is not successful, then an error is returned.
	ClusterCredentials(ctx context.Context, cfg ClusterCredentialsConfig) (*rest.Config, time.Time, error)
}

// aksClient wraps the ARMAKS API and satisfies AKSClient.
type aksClient struct {
	api        ARMAKS
	azIdentity azureIdentityFunction
}

// NewAKSClustersClient returns a client for Azure AKS clusters.
func NewAKSClustersClient(api ARMAKS, azIdentity azureIdentityFunction) AKSClient {
	if azIdentity == nil {
		azIdentity = func(options *azidentity.DefaultAzureCredentialOptions) (GetToken, error) {
			cc, err := azidentity.NewDefaultAzureCredential(options)
			return cc, err
		}
	}
	return &aksClient{api: api, azIdentity: azIdentity}
}

// get returns AKSCluster information for a single AKS cluster.
func (c *aksClient) get(ctx context.Context, group, name string) (*AKSCluster, error) {
	res, err := c.api.Get(ctx, group, name, nil)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}
	cluster, err := AKSClusterFromManagedCluster(&res.ManagedCluster)
	return cluster, trace.Wrap(err)
}

func (c *aksClient) ListAll(ctx context.Context) ([]*AKSCluster, error) {
	var servers []*AKSCluster
	options := &armcontainerservice.ManagedClustersClientListOptions{}
	pager := c.api.NewListPager(options)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, s := range page.Value {
			cluster, err := AKSClusterFromManagedCluster(s)
			if err != nil {
				slog.DebugContext(ctx, "Failed to convert discovered AKS cluster to Teleport internal representation",
					"cluster", StringVal(s.Name),
					"error", err,
				)
				continue
			}
			servers = append(servers, cluster)

		}
	}
	return servers, nil
}

func (c *aksClient) ListWithinGroup(ctx context.Context, group string) ([]*AKSCluster, error) {
	var servers []*AKSCluster
	options := &armcontainerservice.ManagedClustersClientListByResourceGroupOptions{}
	pager := c.api.NewListByResourceGroupPager(group, options)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(ConvertResponseError(err))
		}
		for _, s := range page.Value {
			cluster, err := AKSClusterFromManagedCluster(s)
			if err != nil {
				slog.DebugContext(ctx, "Failed to convert discovered AKS cluster to Teleport internal representation",
					"cluster", StringVal(s.Name),
					"error", err,
				)
				continue
			}
			servers = append(servers, cluster)
		}
	}
	return servers, nil
}

type ClientConfig struct {
	ResourceGroup string
	Name          string
	TenantID      string
}

func (c *aksClient) ClusterCredentials(ctx context.Context, cfg ClusterCredentialsConfig) (*rest.Config, time.Time, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	// get cluster auth information
	clusterDetails, err := c.get(ctx, cfg.ResourceGroup, cfg.ResourceName)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(ConvertResponseError(err))
	}

	switch clusterDetails.Properties.AccessConfig {
	case AzureRBAC:
		// In this mode, Authentication happens via AD users and Authorization is granted by AzureRBAC.
		cfg, expiresOn, err := c.getAzureRBACCredentials(ctx, cfg)
		return cfg, expiresOn, trace.Wrap(err)
	case AzureAD:
		// In this mode, Authentication happens via AD users and Authorization is granted by Kubernetes RBAC.
		cfg, expiresOn, err := c.getAzureADCredentials(ctx, cfg)
		return cfg, expiresOn, trace.Wrap(err)
	case LocalAccounts:
		// In this mode, Authentication is granted by provisioned static accounts accessible via
		// ListClusterUserCredentials
		cfg, err := c.getUserCredentials(ctx, cfg)
		if err != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}
		// make sure the credentials are not exec based.
		cfg, err = checkIfAuthMethodIsUnSupported(cfg)
		// the access credentials are static and are only changed if there is a change in the cluster CA, however to prevent this we will refresh the credentials
		return cfg, time.Now().Add(1 * time.Hour), trace.Wrap(err)
	default:
		return nil, time.Time{}, trace.BadParameter("unsupported AKS authentication mode %v", clusterDetails.Properties.AccessConfig)
	}
}

// getAzureRBACCredentials generates a config to access the cluster.
// When AzureRBAC is enabled, the authentication happens with a BearerToken and the agent's Active Directory
// group has the access rules to access the cluster. If checkPermissions fails we cannot do anything
// and the user has to manually edit the agent's group permissions.
func (c *aksClient) getAzureRBACCredentials(ctx context.Context, cluster ClusterCredentialsConfig) (*rest.Config, time.Time, error) {
	cfg, err := c.getUserCredentials(ctx, cluster)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	expiresOn, err := c.getAzureToken(ctx, cluster.TenantID, cfg)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	if err := c.checkAccessPermissions(ctx, cfg, cluster); err != nil {
		return nil, time.Time{}, trace.WrapWithMessage(err, `Azure RBAC rules have not been configured for the agent.
		Please check that you have configured them correctly.`)
	}

	return cfg, expiresOn, nil
}

// getUserCredentials gets the user credentials by calling `ListClusterUserCredentials` method
// and parsing the kubeconfig returned.
func (c *aksClient) getUserCredentials(ctx context.Context, cfg ClusterCredentialsConfig) (*rest.Config, error) {
	options := &armcontainerservice.ManagedClustersClientListClusterUserCredentialsOptions{
		// format is only applied if AD is enabled but we can force the request with it.
		Format: to.Ptr(armcontainerservice.FormatExec),
	}
	res, err := c.api.ListClusterUserCredentials(ctx, cfg.ResourceGroup, cfg.ResourceName, options)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	result, err := c.getRestConfigFromKubeconfigs(res.Kubeconfigs)
	return result, trace.Wrap(err)
}

// getAzureADCredentials gets the client configuration and checks if Kubernetes RBAC is configured.
func (c *aksClient) getAzureADCredentials(ctx context.Context, cluster ClusterCredentialsConfig) (*rest.Config, time.Time, error) {
	// getUserCredentials is used to extract the cluster CA and API endpoint.
	cfg, err := c.getUserCredentials(ctx, cluster)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}
	expiresOn, err := c.getAzureToken(ctx, cluster.TenantID, cfg)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	// checks if agent already has access to the cluster
	if err := c.checkAccessPermissions(ctx, cfg, cluster); err == nil {
		// access to the cluster was already granted!
		return cfg, expiresOn, nil
	}

	// parse the azure JWT token to extract the first groupID the principal belongs to.
	groupID, err := extractGroupFromAzure(cfg.BearerToken)
	if err != nil {
		return nil, time.Time{}, trace.Wrap(err)
	}

	var (
		adminCredentialsErr error
		runCMDErr           error
	)

	// calls the ListClusterAdminCrdentials endpoint to return the admin static credentials.
	adminCfg, err := c.getAdminCredentials(ctx, cluster.ResourceGroup, cluster.ResourceName)
	switch err {
	case nil:
		// given the admin credentials, the agent will try to create the ClusterRole and
		// ClusterRoleBinding objects in the AKS cluster.
		if adminCredentialsErr = c.grantAccessWithAdminCredentials(ctx, adminCfg, groupID); adminCredentialsErr == nil {
			// checks if agent already has access to the cluster
			if err := c.checkAccessPermissions(ctx, cfg, cluster); err == nil {
				// access to the cluster was already granted!
				return cfg, expiresOn, nil
			}
		}
		adminCredentialsErr = trace.WrapWithMessage(adminCredentialsErr, `Tried to grant access to %s/%s using aks.ListClusterAdminCredentials`, cluster.ResourceGroup, cluster.ResourceName)
		// if the creation failed, then the agent will try to run a command to create them.
		fallthrough
	default:
		if runCMDErr = c.grantAccessWithCommand(ctx, cluster.ResourceGroup, cluster.ResourceName, cluster.TenantID, groupID); runCMDErr != nil {
			return nil, time.Time{}, trace.Wrap(err)
		}
		if err := c.checkAccessPermissions(ctx, cfg, cluster); err == nil {
			// access to the cluster was already granted!
			return cfg, expiresOn, nil
		}
		runCMDErr = trace.WrapWithMessage(runCMDErr, `Tried to grant access to %s/%s using aks.BeginRunCommand`, cluster.ResourceGroup, cluster.ResourceName)
		return nil, time.Time{}, trace.WrapWithMessage(trace.NewAggregate(adminCredentialsErr, runCMDErr), `Cannot grant access to %s/%s AKS cluster`, cluster.ResourceGroup, cluster.ResourceName)
	}

}

// getAdminCredentials returns the cluster admin credentials by calling ListClusterAdminCredentials method.
// This function also validates if the credentials are not exec based.
func (c *aksClient) getAdminCredentials(ctx context.Context, group, name string) (*rest.Config, error) {
	options := &armcontainerservice.ManagedClustersClientListClusterAdminCredentialsOptions{}
	res, err := c.api.ListClusterAdminCredentials(ctx, group, name, options)
	if err != nil {
		return nil, trace.Wrap(ConvertResponseError(err))
	}

	result, err := c.getRestConfigFromKubeconfigs(res.Kubeconfigs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result, err = checkIfAuthMethodIsUnSupported(result)
	return result, trace.Wrap(err)
}

// getRestConfigFromKubeconfigs parses the first kubeConfig returned by ListClusterAdminCredentials and
// ListClusterUserCredentials methods.
func (c *aksClient) getRestConfigFromKubeconfigs(kubes []*armcontainerservice.CredentialResult) (*rest.Config, error) {
	if len(kubes) == 0 {
		return nil, trace.NotFound("no valid kubeconfig returned")
	}
	config, err := clientcmd.Load(kubes[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeRestConfig, err := clientcmd.NewDefaultClientConfig(*config, nil).ClientConfig()
	return kubeRestConfig, trace.Wrap(err)
}

// checkAccessPermissions checks if the agent has the required permissions to operate.
func (c *aksClient) checkAccessPermissions(ctx context.Context, clientCfg *rest.Config, cfg ClusterCredentialsConfig) error {
	client, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return trace.Wrap(err, "failed to generate Kubernetes client for cluster")
	}
	sarClient := client.AuthorizationV1().SelfSubjectAccessReviews()
	return trace.Wrap(cfg.ImpersonationPermissionsChecker(ctx, cfg.ResourceName, sarClient))
}

// getAzureToken generates an authentication token and changes the rest.Config.
func (c *aksClient) getAzureToken(ctx context.Context, tentantID string, clientCfg *rest.Config) (time.Time, error) {
	token, time, err := c.genAzureToken(ctx, tentantID)
	if err != nil {
		return time, trace.Wrap(err)
	}
	// reset the old exec provider credentials
	clientCfg.ExecProvider = nil
	clientCfg.BearerToken = token

	return time, nil
}

// genAzureToken generates an authentication token for clusters with AD enabled.
func (c *aksClient) genAzureToken(ctx context.Context, tentantID string) (string, time.Time, error) {
	const (
		// azureManagedClusterScope is a fixed uuid used to inform Azure
		// that we want a Token fully populated with identity principals.
		// ref: https://github.com/Azure/kubelogin#exec-plugin-format
		azureManagedClusterScope = "6dae42f8-4368-4678-94ff-3960e28e3630"
	)
	cred, err := c.azIdentity(&azidentity.DefaultAzureCredentialOptions{
		TenantID: tentantID,
	})
	if err != nil {
		return "", time.Time{}, trace.Wrap(ConvertResponseError(err))
	}

	cliAccessToken, origErr := cred.GetToken(ctx, policy.TokenRequestOptions{
		// azureManagedClusterScope is a fixed scope that identifies azure AKS managed clusters.
		Scopes: []string{azureManagedClusterScope},
	},
	)
	if origErr == nil {
		return cliAccessToken.Token, cliAccessToken.ExpiresOn, nil
	}

	// Some azure credentials like Workload Identity - but not all - require the
	// scope to be suffixed with /.default.
	// Since the AZ identity returns a chained credentials provider
	// that tries to get the token from any of the configured providers but doesn't
	// expose which provider was used, we retry the token generation with the
	// the expected scope.
	// In the case of this attempt doesn't return any valid credential, we return
	// the original error.
	cliAccessToken, err = cred.GetToken(
		ctx,
		policy.TokenRequestOptions{
			// azureManagedClusterScope is a fixed scope that identifies azure AKS managed clusters.
			Scopes: []string{azureManagedClusterScope + "/.default"},
		},
	)
	if err != nil {
		// use the original error since it's clear.
		return "", time.Time{}, trace.Wrap(ConvertResponseError(origErr))
	}
	return cliAccessToken.Token, cliAccessToken.ExpiresOn, nil
}

// grantAccessWithAdminCredentials tries to create the ClusterRole and ClusterRoleBinding into the AKS cluster
// using admin credentials.
func (c *aksClient) grantAccessWithAdminCredentials(ctx context.Context, adminCfg *rest.Config, groupID string) error {
	client, err := kubernetes.NewForConfig(adminCfg)
	if err != nil {
		return trace.Wrap(err, "failed to generate Kubernetes client for cluster")
	}

	if err := c.upsertClusterRoleWithAdminCredentials(ctx, client); err != nil {
		return trace.Wrap(err)
	}

	err = c.upsertClusterRoleBindingWithAdminCredentials(ctx, client, groupID)
	return trace.Wrap(err)
}

// upsertClusterRoleWithAdminCredentials tries to upsert the ClusterRole using admin credentials.
func (c *aksClient) upsertClusterRoleWithAdminCredentials(ctx context.Context, client *kubernetes.Clientset) error {
	clusterRole := &v1.ClusterRole{}

	if err := yaml.Unmarshal([]byte(fixtures.KubeClusterRoleTemplate), clusterRole); err != nil {
		return trace.Wrap(err)
	}

	_, err := client.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{
		FieldManager: resourceOwner,
	})
	if err == nil {
		return nil
	}

	if kubeerrors.IsAlreadyExists(err) {
		_, err := client.RbacV1().ClusterRoles().Update(ctx, clusterRole, metav1.UpdateOptions{
			FieldManager: resourceOwner,
		})
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

// upsertClusterRoleBindingWithAdminCredentials tries to upsert the ClusterRoleBinding using admin credentials
// and maps it into the principal group.
func (c *aksClient) upsertClusterRoleBindingWithAdminCredentials(ctx context.Context, client *kubernetes.Clientset, groupID string) error {
	clusterRoleBinding := &v1.ClusterRoleBinding{}

	if err := yaml.Unmarshal([]byte(fixtures.KubeClusterRoleBindingTemplate), clusterRoleBinding); err != nil {
		return trace.Wrap(err)
	}

	if len(clusterRoleBinding.Subjects) == 0 {
		return trace.BadParameter("Subjects field were not correctly unmarshaled")
	}

	clusterRoleBinding.Subjects[0].Name = groupID

	_, err := client.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{
		FieldManager: resourceOwner,
	})
	if err == nil {
		return nil
	}

	if kubeerrors.IsAlreadyExists(err) {
		_, err := client.RbacV1().ClusterRoleBindings().Update(ctx, clusterRoleBinding, metav1.UpdateOptions{
			FieldManager: resourceOwner,
		})
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

// grantAccessWithCommand tries to create the ClusterRole and ClusterRoleBinding into the AKS cluster
// using remote kubectl command.
func (c *aksClient) grantAccessWithCommand(ctx context.Context, resourceGroupName, resourceName, tentantID, groupID string) error {
	token, _, err := c.genAzureToken(ctx, tentantID)
	if err != nil {
		return trace.Wrap(err)
	}
	cmd, err := c.api.BeginRunCommand(ctx, resourceGroupName, resourceName, armcontainerservice.RunCommandRequest{
		ClusterToken: to.Ptr(token),
		Command:      to.Ptr(kubectlApplyString(groupID)),
	}, &armcontainerservice.ManagedClustersClientBeginRunCommandOptions{})
	if err != nil {
		return trace.Wrap(ConvertResponseError(err))
	}
	_, err = cmd.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{Frequency: time.Second})
	return trace.Wrap(ConvertResponseError(err))
}

// extractGroupFromAzure extracts the first group ID from the Azure Bearer Token.
func extractGroupFromAzure(token string) (string, error) {
	p := jwt.NewParser()
	claims := &azureGroupClaims{}
	// We do not want to validate the token since
	// we generated it from Azure SDK.
	_, _, err := p.ParseUnverified(token, claims)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if len(claims.Groups) == 0 {
		return "", trace.BadParameter("no groups found in Azure token")
	}

	return claims.Groups[0], nil
}

// checkIfAuthMethodIsUnSupported checks if the credentials are not exec based.
func checkIfAuthMethodIsUnSupported(cfg *rest.Config) (*rest.Config, error) {
	if cfg.ExecProvider != nil {
		return nil, trace.BadParameter("exec auth format not supported")
	}
	return cfg, nil
}

// AKSClusterFromManagedCluster converts an Azure armcontainerservice.ManagedCluster into AKSCluster.
func AKSClusterFromManagedCluster(cluster *armcontainerservice.ManagedCluster) (*AKSCluster, error) {
	result := &AKSCluster{
		Name:     StringVal(cluster.Name),
		Location: StringVal(cluster.Location),
		Tags:     ConvertTags(cluster.Tags),
	}
	if cluster.Identity != nil {
		result.TenantID = StringVal(cluster.Identity.TenantID)
	}
	if subID, groupName, err := extractSubscriptionAndGroupName(cluster.ID); err == nil {
		result.GroupName, result.SubscriptionID = groupName, subID
	}

	if cluster.Properties == nil {
		return nil, trace.BadParameter("invalid AKS Cluster Properties")
	}

	if !isAKSClusterRunning(cluster.Properties) {
		return nil, trace.BadParameter("AKS cluster not running")
	}

	if cluster.Properties.AADProfile != nil && ptrToVal(cluster.Properties.AADProfile.EnableAzureRBAC) {
		result.Properties = AKSClusterProperties{
			AccessConfig: AzureRBAC,
		}
	} else if cluster.Properties.AADProfile != nil {
		result.Properties = AKSClusterProperties{
			AccessConfig:  AzureAD,
			LocalAccounts: !ptrToVal(cluster.Properties.DisableLocalAccounts),
		}
	} else {
		result.Properties = AKSClusterProperties{
			AccessConfig:  LocalAccounts,
			LocalAccounts: true,
		}
	}

	return result, nil
}

func ptrToVal[T any](ptr *T) T {
	var result T
	if ptr != nil {
		result = *ptr
	}
	return result
}

// extractSubscriptionAndGroupName extracts the group name and subscription id from resource id.
// ids are in the form of:
// /subscriptions/{subscription_id}/resourcegroups/{resource_group}/providers/Microsoft.ContainerService/managedClusters/{name}
func extractSubscriptionAndGroupName(id *string) (string, string, error) {
	if id == nil {
		return "", "", trace.BadParameter("invalid resource_id provided")
	}
	resource, err := armazcore.ParseResourceID(*id)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	return resource.SubscriptionID, resource.ResourceGroupName, nil
}

// azureGroupClaims the configuration settings of the Azure Active Directory allowed principals.
type azureGroupClaims struct {
	// Groups - The list of the allowed groups.
	Groups []string `json:"groups,omitempty"`
}

func (c *azureGroupClaims) Valid() error {
	if len(c.Groups) == 0 {
		return trace.BadParameter("invalid claims received")
	}
	return nil
}

func isAKSClusterRunning(properties *armcontainerservice.ManagedClusterProperties) bool {
	if properties.PowerState != nil && properties.PowerState.Code != nil &&
		*properties.PowerState.Code == armcontainerservice.CodeRunning {
		return true
	}
	return false
}

// kubectlApplyString generates a kubectl apply command to create the ClusterRole
// and ClusterRoleBinding.
// cat <<EOF | kubectl apply -f -
// apiVersion: rbac.authorization.k8s.io/v1
// kind: ClusterRole
// metadata:
//
//	name: teleport
//
// rules:
// - apiGroups:
//   - ""
//     resources:
//   - users
//   - groups
//   - serviceaccounts
//     verbs:
//   - impersonate
//
// - apiGroups:
//   - ""
//     resources:
//   - pods
//     verbs:
//   - get
//
// - apiGroups:
//   - "authorization.k8s.io"
//     resources:
//   - selfsubjectaccessreviews
//   - selfsubjectrulesreviews
//     verbs:
//   - create
//
// ---
// apiVersion: rbac.authorization.k8s.io/v1
// kind: ClusterRoleBinding
// metadata:
//
//	name: teleport
//
// roleRef:
//
//	apiGroup: rbac.authorization.k8s.io
//	kind: ClusterRole
//	name: teleport
//
// subjects:
//   - kind: Group
//     name: group
//     apiGroup: rbac.authorization.k8s.io
//
// EOF
func kubectlApplyString(group string) string {
	return fmt.Sprintf(`cat <<EOF | kubectl apply -f -
%s
---
%s
EOF`, fixtures.KubeClusterRoleTemplate, strings.ReplaceAll(fixtures.KubeClusterRoleBindingTemplate, "group_name", group))
}
