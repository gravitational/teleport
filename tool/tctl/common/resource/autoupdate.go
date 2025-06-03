package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	config, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if config.GetMetadata() == nil {
		config.Metadata = &headerv1.Metadata{}
	}
	if config.GetMetadata().GetName() == "" {
		config.Metadata.Name = types.MetaNameAutoUpdateConfig
	}

	if rc.IsForced() {
		_, err = client.UpsertAutoUpdateConfig(ctx, config)
	} else {
		_, err = client.CreateAutoUpdateConfig(ctx, config)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_config has been created")
	return nil
}

func (rc *ResourceCommand) updateAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	config, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateConfig](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if config.GetMetadata() == nil {
		config.Metadata = &headerv1.Metadata{}
	}
	if config.GetMetadata().GetName() == "" {
		config.Metadata.Name = types.MetaNameAutoUpdateConfig
	}

	if _, err := client.UpdateAutoUpdateConfig(ctx, config); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("autoupdate_config has been updated")
	return nil
}

func (rc *ResourceCommand) getAutoUpdateConfig(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	config, err := client.GetAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAutoUpdateConfigCollection(config), nil
}

func (rc *ResourceCommand) createAutoUpdateVersion(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	version, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateVersion](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if version.GetMetadata() == nil {
		version.Metadata = &headerv1.Metadata{}
	}
	if version.GetMetadata().GetName() == "" {
		version.Metadata.Name = types.MetaNameAutoUpdateVersion
	}

	if rc.IsForced() {
		_, err = client.UpsertAutoUpdateVersion(ctx, version)
	} else {
		_, err = client.CreateAutoUpdateVersion(ctx, version)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_version has been created")
	return nil
}

func (rc *ResourceCommand) updateAutoUpdateVersion(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	version, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateVersion](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if version.GetMetadata() == nil {
		version.Metadata = &headerv1.Metadata{}
	}
	if version.GetMetadata().GetName() == "" {
		version.Metadata.Name = types.MetaNameAutoUpdateVersion
	}

	if _, err := client.UpdateAutoUpdateVersion(ctx, version); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("autoupdate_version has been updated")
	return nil
}

func (rc *ResourceCommand) getAutoUpdateVersion(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	version, err := client.GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAutoUpdateVersionCollection(version), nil
}

func (rc *ResourceCommand) createAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	rollout, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateAgentRollout](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rollout.GetMetadata() == nil {
		rollout.Metadata = &headerv1.Metadata{}
	}
	if rollout.GetMetadata().GetName() == "" {
		rollout.Metadata.Name = types.MetaNameAutoUpdateAgentRollout
	}

	if rc.IsForced() {
		_, err = client.UpsertAutoUpdateAgentRollout(ctx, rollout)
	} else {
		_, err = client.CreateAutoUpdateAgentRollout(ctx, rollout)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_agent_rollout has been created")
	return nil
}

func (rc *ResourceCommand) getAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	version, err := client.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAutoUpdateAgentRolloutCollection(version), nil
}

func (rc *ResourceCommand) upsertAutoUpdateAgentReport(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	report, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateAgentReport](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = client.UpsertAutoUpdateAgentReport(ctx, report)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("autoupdate_agent_report has been created")
	return nil
}

func (rc *ResourceCommand) getAutoUpdateAgentReport(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		report, err := client.GetAutoUpdateAgentReport(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewAutoUpdateAgentReportCollection([]*autoupdatev1pb.AutoUpdateAgentReport{report}), nil
	}

	var reports []*autoupdatev1pb.AutoUpdateAgentReport
	var nextToken string
	for {
		resp, token, err := client.ListAutoUpdateAgentReports(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, resp...)
		if token == "" {
			break
		}
		nextToken = token
	}
	return collections.NewAutoUpdateAgentReportCollection(reports), nil
}

func (rc *ResourceCommand) updateAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	rollout, err := services.UnmarshalProtoResource[*autoupdatev1pb.AutoUpdateAgentRollout](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rollout.GetMetadata() == nil {
		rollout.Metadata = &headerv1.Metadata{}
	}
	if rollout.GetMetadata().GetName() == "" {
		rollout.Metadata.Name = types.MetaNameAutoUpdateAgentRollout
	}

	if _, err := client.UpdateAutoUpdateAgentRollout(ctx, rollout); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("autoupdate_version has been updated")
	return nil
}
