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

func (rc *ResourceCommand) getUIConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindUIConfig)
	}
	uiconfig, err := client.GetUIConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewUIConfigCollection(uiconfig), nil
}

func (rc *ResourceCommand) createUIConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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

func resetAuthPreference(ctx context.Context, client *authclient.Client) error {
	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedAuthPref.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	return trace.Wrap(client.ResetAuthPreference(ctx))
}

func resetClusterNetworkingConfig(ctx context.Context, client *authclient.Client) error {
	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedNetConfig.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	return trace.Wrap(client.ResetClusterNetworkingConfig(ctx))
}

func resetSessionRecordingConfig(ctx context.Context, client *authclient.Client) error {
	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	managedByStaticConfig := storedRecConfig.Origin() == types.OriginConfigFile
	if managedByStaticConfig {
		return trace.BadParameter("%s", managedByStaticDeleteMsg)
	}

	return trace.Wrap(client.ResetSessionRecordingConfig(ctx))
}

func resetNetworkRestrictions(ctx context.Context, client *authclient.Client) error {
	return trace.Wrap(client.DeleteNetworkRestrictions(ctx))
}

func (rc *ResourceCommand) getAuthPreference(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterAuthPreference)
	}
	authPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAuthPreferenceCollection(authPref), nil
}

// createAuthPreference implements `tctl create cap.yaml` command.
func (rc *ResourceCommand) createAuthPreference(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	newAuthPref, err := services.UnmarshalAuthPreference(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedAuthPref, "cluster auth preference", rc.force, rc.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertAuthPreference(ctx, newAuthPref); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been created\n")
	return nil
}

func (rc *ResourceCommand) updateAuthPreference(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	newAuthPref, err := services.UnmarshalAuthPreference(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedAuthPref, err := client.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedAuthPref, "cluster auth preference", rc.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateAuthPreference(ctx, newAuthPref); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster auth preference has been updated\n")
	return nil
}

func (rc *ResourceCommand) getNetworkRestrictions(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	nr, err := client.GetNetworkRestrictions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewNetworkRestrictionCollection(nr), nil
}

func (rc *ResourceCommand) getClusterNetworkingConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterNetworkingConfig)
	}
	netConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewNetworkConfigCollection(netConfig), nil
}

// createClusterNetworkingConfig implements `tctl create netconfig.yaml` command.
func (rc *ResourceCommand) createClusterNetworkingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	newNetConfig, err := services.UnmarshalClusterNetworkingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedNetConfig, "cluster networking configuration", rc.force, rc.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertClusterNetworkingConfig(ctx, newNetConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been updated\n")
	return nil
}

// updateClusterNetworkingConfig
func (rc *ResourceCommand) updateClusterNetworkingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	newNetConfig, err := services.UnmarshalClusterNetworkingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedNetConfig, err := client.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedNetConfig, "cluster networking configuration", rc.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateClusterNetworkingConfig(ctx, newNetConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cluster networking configuration has been updated\n")
	return nil
}

func (rc *ResourceCommand) getClusterMaintenanceConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindClusterMaintenanceConfig)
	}

	cmc, err := client.GetClusterMaintenanceConfig(ctx)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	return collections.NewMaintenanceWindowCollection(cmc), nil
}

func (rc *ResourceCommand) createClusterMaintenanceConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	var cmc types.ClusterMaintenanceConfigV1
	if err := utils.FastUnmarshal(raw.Raw, &cmc); err != nil {
		return trace.Wrap(err)
	}

	if err := cmc.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if rc.force {
		// max nonce forces "upsert" behavior
		cmc.Nonce = math.MaxUint64
	}

	if err := client.UpdateClusterMaintenanceConfig(ctx, &cmc); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("maintenance window has been updated")
	return nil
}

func (rc *ResourceCommand) getSessionRecordingConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		return nil, trace.BadParameter("only simple `tctl get %v` can be used", types.KindSessionRecordingConfig)
	}
	recConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewRecConfigCollection(recConfig), nil
}

// createSessionRecordingConfig implements `tctl create recconfig.yaml` command.
func (rc *ResourceCommand) createSessionRecordingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	newRecConfig, err := services.UnmarshalSessionRecordingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCreateResourceWithOrigin(storedRecConfig, "session recording configuration", rc.force, rc.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpsertSessionRecordingConfig(ctx, newRecConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("session recording configuration has been updated\n")
	return nil
}

func (rc *ResourceCommand) updateSessionRecordingConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	newRecConfig, err := services.UnmarshalSessionRecordingConfig(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	storedRecConfig, err := client.GetSessionRecordingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkUpdateResourceWithOrigin(storedRecConfig, "session recording configuration", rc.confirm); err != nil {
		return trace.Wrap(err)
	}

	if _, err := client.UpdateSessionRecordingConfig(ctx, newRecConfig); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("session recording configuration has been updated\n")
	return nil
}

// createNetworkRestrictions implements `tctl create net_restrict.yaml` command.
func (rc *ResourceCommand) createNetworkRestrictions(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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
