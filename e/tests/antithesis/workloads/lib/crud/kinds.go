package crud

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// DefaultKinds returns the Teleport kinds supported by OpsForKind.
//
// This list intentionally contains only resource kinds that have ordinary
// name-based CRUD APIs, can be created from small synthetic specs, and are safe
// for the fuzzer to create, update, and delete by random test-prefixed names.
//
// Skipped resources fall into a few categories:
//   - singleton or cluster-wide config, like auth preference, cluster networking,
//     session recording, backend info, VNet config, Beams config, and auto-update
//     resources, because deletes or updates would mutate real cluster state.
//   - runtime or heartbeat-owned resources, like nodes, app/database/kube servers,
//     desktop services, proxy/auth servers, sessions, reports, user
//     tasks, notifications
//   - workflow or composite resources, like tokens, access requests, locks,
//     access lists, plugins, plugin credentials, workload identities, scoped
//     access, and signing policies
//
// TODO(okraport): Expand the list of of supported types.
func DefaultKinds() []string {
	return []string{
		types.KindRole,
		types.KindApp,
		types.KindDatabase,
		types.KindKubernetesCluster,
		types.KindAccessMonitoringRule,
		types.KindCrownJewel,
		types.KindDiscoveryConfig,
		types.KindDynamicWindowsDesktop,
		types.KindHealthCheckConfig,
		types.KindLinuxDesktop,
		types.KindStaticHostUser,
		types.KindUser,
		types.KindUserGroup,
		types.KindWindowsDesktop,
		// TODO(okraport): add IsEqual support to types.Integration and enable this kind.
		// types.KindIntegration,
		// TODO(okraport): add IsEqual support to types.SAMLIdPServiceProvider and enable this kind.
		// types.KindSAMLIdPServiceProvider,
	}
}

func DefaultKindsWithModernUpdateSemantics() []string {
	return []string{
		types.KindCrownJewel,
		types.KindAccessMonitoringRule,
		types.KindDiscoveryConfig,
		types.KindDynamicWindowsDesktop,
		types.KindHealthCheckConfig,
		types.KindLinuxDesktop,
		types.KindRole,
		types.KindStaticHostUser,
		types.KindUser,
		// TODO(okraport): add IsEqual support to types.Integration and enable this kind.
		// types.KindIntegration,
	}
}

func DefaultKindsWithConditionalUpdate() []string {
	return []string{
		types.KindRole,
		types.KindCrownJewel,
		types.KindDynamicWindowsDesktop,
		types.KindHealthCheckConfig,
		types.KindLinuxDesktop,
		types.KindStaticHostUser,
		types.KindUser,
	}
}

// OpsForKind returns CRUD operations for a given Teleport kind.
func OpsForKind(client any, kind string) (KindResourceOps, error) {
	switch kind {
	case types.KindRole:
		return RoleOpsForClient(client)
	case types.KindApp:
		return AppOpsForClient(client)
	case types.KindDatabase:
		return DatabaseOpsForClient(client)
	case types.KindKubernetesCluster:
		return KubernetesClusterOpsForClient(client)
	case types.KindAccessMonitoringRule:
		return AccessMonitoringRuleOpsForClient(client)
	case types.KindCrownJewel:
		return CrownJewelOpsForClient(client)
	case types.KindDiscoveryConfig:
		return DiscoveryConfigOpsForClient(client)
	case types.KindDynamicWindowsDesktop:
		return DynamicWindowsDesktopOpsForClient(client)
	case types.KindHealthCheckConfig:
		return HealthCheckConfigOpsForClient(client)
	case types.KindLinuxDesktop:
		return LinuxDesktopOpsForClient(client)
	case types.KindStaticHostUser:
		return StaticHostUserOpsForClient(client)
	case types.KindUser:
		return UserOpsForClient(client)
	case types.KindUserGroup:
		return UserGroupOpsForClient(client)
	case types.KindWindowsDesktop:
		return WindowsDesktopOpsForClient(client)
	// TODO(okraport): add IsEqual support to types.Integration and enable this kind.
	// case types.KindIntegration:
	// 	return IntegrationOpsForClient(client)
	// TODO(okraport): add IsEqual support to types.SAMLIdPServiceProvider and enable this kind.
	// case types.KindSAMLIdPServiceProvider:
	// 	return SAMLIdPServiceProviderOpsForClient(client)
	default:
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", kind)
	}
}
