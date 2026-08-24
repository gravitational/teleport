package crud_test

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	authpb "github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

type fakeClient struct{}

func (fakeClient) CreateRole(context.Context, types.Role) (types.Role, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetRoles(context.Context) ([]types.Role, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetRole(context.Context, string) (types.Role, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListRoles(context.Context, *authpb.ListRolesRequest) (*authpb.ListRolesResponse, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateRole(context.Context, types.Role) (types.Role, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertRole(context.Context, types.Role) (types.Role, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteRole(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) GetLock(context.Context, string) (types.Lock, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetLocks(context.Context, bool, ...types.LockTarget) ([]types.Lock, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListLocks(context.Context, int, string, *types.LockFilter) ([]types.Lock, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) RangeLocks(context.Context, string, string, *types.LockFilter) iter.Seq2[types.Lock, error] {
	return func(yield func(types.Lock, error) bool) {}
}

func (fakeClient) UpsertLock(context.Context, types.Lock) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteLock(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ReplaceRemoteLocks(context.Context, string, []types.Lock) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) CreateApp(context.Context, types.Application) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) GetApps(context.Context) ([]types.Application, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetApp(context.Context, string) (types.Application, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListApps(context.Context, int, string) ([]types.Application, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) Apps(context.Context, string, string) iter.Seq2[types.Application, error] {
	return func(yield func(types.Application, error) bool) {}
}

func (fakeClient) UpdateApp(context.Context, types.Application) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteApp(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllApps(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) CreateDatabase(context.Context, types.Database) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) GetDatabases(context.Context) ([]types.Database, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetDatabase(context.Context, string) (types.Database, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListDatabases(context.Context, int, string) ([]types.Database, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) RangeDatabases(context.Context, string, string) iter.Seq2[types.Database, error] {
	return func(yield func(types.Database, error) bool) {}
}

func (fakeClient) UpdateDatabase(context.Context, types.Database) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteDatabase(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllDatabases(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) CreateKubernetesCluster(context.Context, types.KubeCluster) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) GetKubernetesClusters(context.Context) ([]types.KubeCluster, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetKubeCluster(context.Context, *presencev1.GetKubeClusterRequest) (types.KubeCluster, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListKubeClusters(context.Context, *presencev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) RangeKubeClusters(context.Context, *presencev1.ListKubeClustersRequest) iter.Seq2[types.KubeCluster, error] {
	return func(yield func(types.KubeCluster, error) bool) {}
}

func (fakeClient) UpdateKubernetesCluster(context.Context, types.KubeCluster) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteKubeCluster(context.Context, *presencev1.DeleteKubeClusterRequest) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllKubernetesClusters(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) CreateAccessMonitoringRule(context.Context, *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateAccessMonitoringRule(context.Context, *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertAccessMonitoringRule(context.Context, *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetAccessMonitoringRule(context.Context, string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteAccessMonitoringRule(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllAccessMonitoringRules(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListAccessMonitoringRules(context.Context, int, string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) ListAccessMonitoringRulesWithFilter(context.Context, *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) ListCrownJewels(context.Context, int64, string) ([]*crownjewelv1.CrownJewel, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetCrownJewel(context.Context, string) (*crownjewelv1.CrownJewel, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertCrownJewel(context.Context, *crownjewelv1.CrownJewel) (*crownjewelv1.CrownJewel, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteCrownJewel(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) CreateDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertDiscoveryConfig(context.Context, *discoveryconfig.DiscoveryConfig) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteDiscoveryConfig(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllDiscoveryConfigs(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListDiscoveryConfigs(context.Context, int, string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetDiscoveryConfig(context.Context, string) (*discoveryconfig.DiscoveryConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetDynamicWindowsDesktop(context.Context, string) (types.DynamicWindowsDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertDynamicWindowsDesktop(context.Context, types.DynamicWindowsDesktop) (types.DynamicWindowsDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteDynamicWindowsDesktop(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListDynamicWindowsDesktops(context.Context, int, string) ([]types.DynamicWindowsDesktop, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) ListGitServers(context.Context, int, string) ([]types.Server, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetGitServer(context.Context, string) (types.Server, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateGitServer(context.Context, types.Server) (types.Server, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateGitServer(context.Context, types.Server) (types.Server, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertGitServer(context.Context, types.Server) (types.Server, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteGitServer(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) GetHealthCheckConfig(context.Context, string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListHealthCheckConfigs(context.Context, int, string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) CreateHealthCheckConfig(context.Context, *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateHealthCheckConfig(context.Context, *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertHealthCheckConfig(context.Context, *healthcheckconfigv1.HealthCheckConfig) (*healthcheckconfigv1.HealthCheckConfig, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteHealthCheckConfig(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListIntegrations(context.Context, int, string) ([]types.Integration, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetIntegration(context.Context, string) (types.Integration, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateIntegration(context.Context, types.Integration) (types.Integration, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateIntegration(context.Context, types.Integration) (types.Integration, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteIntegration(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllIntegrations(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) CreateLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertLinuxDesktop(context.Context, *linuxdesktopv1.LinuxDesktop) (*linuxdesktopv1.LinuxDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteLinuxDesktop(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListLinuxDesktops(context.Context, int, string) ([]*linuxdesktopv1.LinuxDesktop, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetLinuxDesktop(context.Context, string) (*linuxdesktopv1.LinuxDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListSAMLIdPServiceProviders(context.Context, int, string) ([]types.SAMLIdPServiceProvider, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetSAMLIdPServiceProvider(context.Context, string) (types.SAMLIdPServiceProvider, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateSAMLIdPServiceProvider(context.Context, types.SAMLIdPServiceProvider) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) UpdateSAMLIdPServiceProvider(context.Context, types.SAMLIdPServiceProvider) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteSAMLIdPServiceProvider(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllSAMLIdPServiceProviders(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListStaticHostUsers(context.Context, int, string) ([]*userprovisioningv2.StaticHostUser, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetStaticHostUser(context.Context, string) (*userprovisioningv2.StaticHostUser, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateStaticHostUser(context.Context, *userprovisioningv2.StaticHostUser) (*userprovisioningv2.StaticHostUser, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateStaticHostUser(context.Context, *userprovisioningv2.StaticHostUser) (*userprovisioningv2.StaticHostUser, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpsertStaticHostUser(context.Context, *userprovisioningv2.StaticHostUser) (*userprovisioningv2.StaticHostUser, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteStaticHostUser(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) CreateUser(context.Context, types.User) (types.User, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) GetUser(context.Context, string, bool) (types.User, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListUsers(context.Context, *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) UpdateUser(context.Context, types.User) (types.User, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) DeleteUser(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListUserGroups(context.Context, int, string) ([]types.UserGroup, string, error) {
	return nil, "", trace.NotImplemented("not used")
}

func (fakeClient) GetUserGroup(context.Context, string) (types.UserGroup, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateUserGroup(context.Context, types.UserGroup) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) UpdateUserGroup(context.Context, types.UserGroup) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteUserGroup(context.Context, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllUserGroups(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) GetWindowsDesktops(context.Context, types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) CreateWindowsDesktop(context.Context, types.WindowsDesktop) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) UpdateWindowsDesktop(context.Context, types.WindowsDesktop) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) UpsertWindowsDesktop(context.Context, types.WindowsDesktop) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteWindowsDesktop(context.Context, string, string) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) DeleteAllWindowsDesktops(context.Context) error {
	return trace.NotImplemented("not used")
}

func (fakeClient) ListWindowsDesktops(context.Context, types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	return nil, trace.NotImplemented("not used")
}

func (fakeClient) ListWindowsDesktopServices(context.Context, types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	return nil, trace.NotImplemented("not used")
}
