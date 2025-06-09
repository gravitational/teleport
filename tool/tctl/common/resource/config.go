package resource

import (
	"context"
	"fmt"
	"math"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var uiConfig = resource{
	getHandler:    getUIConfig,
	createHandler: createUIConfig,
	deleteHandler: deleteUIConfig,
	singleton:     true,
}

func getUIConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindUIConfig)
	}
	uiconfig, err := client.GetUIConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewUIConfigCollection(uiconfig), nil
}

func createUIConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	uic, err := services.UnmarshalUIConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.SetUIConfig(ctx, uic)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("ui_config %q has been set\n", uic.GetName())
	return nil
}

func deleteUIConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	err := client.DeleteUIConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s has been deleted\n", types.KindUIConfig)
	return nil
}

var clusterAuthPreference = resource{
	getHandler:    getAuthPreference,
	createHandler: createAuthPreference,
	updateHandler: updateAuthPreference,
	deleteHandler: resetAuthPreference,
	singleton:     true,
}

func getAuthPreference(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterAuthPreference)
	}
	authPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAuthPreferenceCollection(authPref), nil
}

// createAuthPreference implements `tctl create cap.yaml` command.
func createAuthPreference(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	newAuthPref, err := services.UnmarshalAuthPreference(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedAuthPref, "cluster auth preference", opts.force, opts.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertAuthPreference(ctx, newAuthPref); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been created\n")
	return nil
}

func updateAuthPreference(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	newAuthPref, err := services.UnmarshalAuthPreference(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedAuthPref, "cluster auth preference", opts.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateAuthPreference(ctx, newAuthPref); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been updated\n")
	return nil
}

func resetAuthPreference(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedAuthPref.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	if err := client.ResetAuthPreference(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been reset to defaults\n")
	return nil
}

var clusterNetworkingConfig = resource{
	getHandler:    getClusterNetworkingConfig,
	createHandler: createClusterNetworkingConfig,
	updateHandler: updateClusterNetworkingConfig,
	deleteHandler: resetClusterNetworkingConfig,
	singleton:     true,
}

func getClusterNetworkingConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterNetworkingConfig)
	}
	netConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewNetworkConfigCollection(netConfig), nil
}

// createClusterNetworkingConfig implements `tctl create netconfig.yaml` command.
func createClusterNetworkingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	newNetConfig, err := services.UnmarshalClusterNetworkingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedNetConfig, "cluster networking configuration", opts.force, opts.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertClusterNetworkingConfig(ctx, newNetConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been updated\n")
	return nil
}

// updateClusterNetworkingConfig
func updateClusterNetworkingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	newNetConfig, err := services.UnmarshalClusterNetworkingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedNetConfig, "cluster networking configuration", opts.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateClusterNetworkingConfig(ctx, newNetConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been updated\n")
	return nil
}

func resetClusterNetworkingConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedNetConfig.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	if err := client.ResetClusterNetworkingConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been reset to defaults\n")
	return nil

}

var sessionRecordingConfig = resource{
	getHandler:    getSessionRecordingConfig,
	createHandler: createSessionRecordingConfig,
	updateHandler: updateSessionRecordingConfig,
	deleteHandler: resetSessionRecordingConfig,
	singleton:     true,
}

func getSessionRecordingConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindSessionRecordingConfig)
	}
	recConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewRecConfigCollection(recConfig), nil
}

// createSessionRecordingConfig implements `tctl create recconfig.yaml` command.
func createSessionRecordingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	newRecConfig, err := services.UnmarshalSessionRecordingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedRecConfig, "session recording configuration", opts.force, opts.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertSessionRecordingConfig(ctx, newRecConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("session recording configuration has been updated\n")
	return nil
}

func updateSessionRecordingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	newRecConfig, err := services.UnmarshalSessionRecordingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedRecConfig, "session recording configuration", opts.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateSessionRecordingConfig(ctx, newRecConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("session recording configuration has been updated\n")
	return nil
}

func resetSessionRecordingConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedRecConfig.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	if err := client.ResetSessionRecordingConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("session recording configuration has been reset to defaults\n")
	return nil
}

var networkRestrictions = resource{
	getHandler:    getNetworkRestrictions,
	createHandler: createNetworkRestrictions,
	deleteHandler: resetNetworkRestrictions,
	singleton:     true,
}

func resetNetworkRestrictions(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteNetworkRestrictions(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("network restrictions have been reset to defaults (allow all)\n")
	return nil

}

// createNetworkRestrictions implements `tctl create net_restrict.yaml` command.
func createNetworkRestrictions(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	newNetRestricts, err := services.UnmarshalNetworkRestrictions(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.SetNetworkRestrictions(ctx, newNetRestricts); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("network restrictions have been updated\n")
	return nil
}

func getNetworkRestrictions(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	nr, err := client.GetNetworkRestrictions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewNetworkRestrictionCollection(nr), nil
}

var clusterMaintenanceConfig = resource{
	getHandler:    getClusterMaintenanceConfig,
	createHandler: createClusterMaintenanceConfig,
	deleteHandler: deleteClusterMaintenanceConfig,
	singleton:     true,
}

func getClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterMaintenanceConfig)
	}

	cmc, err := client.GetClusterMaintenanceConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	return collections.NewMaintenanceWindowCollection(cmc), nil
}

func createClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	var cmc types.ClusterMaintenanceConfigV1
	if err := utils.FastUnmarshal(raw.Raw, &cmc); err != nil {
		return trace.Wrap(err)
	}

	if err := cmc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if opts.force {
		// max nonce forces "upsert" behavior
		cmc.Nonce = math.MaxUint64
	}

	if err := client.UpdateClusterMaintenanceConfig(ctx, &cmc); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("maintenance window has been updated")
	return nil
}

func deleteClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteClusterMaintenanceConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster maintenance configuration has been deleted\n")
	return nil
}
