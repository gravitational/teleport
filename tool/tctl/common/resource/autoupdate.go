/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

var autoUpdateConfig = resource{
	getHandler:    getAutoUpdateConfig,
	createHandler: createAutoUpdateConfig,
	updateHandler: updateAutoUpdateConfig,
	deleteHandler: deleteAutoUpdateConfig,
	singleton:     true,
}

func createAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

	if opts.force {
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

func updateAutoUpdateConfig(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteAutoUpdateConfig(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteAutoUpdateConfig(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("AutoUpdateConfig has been deleted\n")
	return nil
}

func getAutoUpdateConfig(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	config, err := client.GetAutoUpdateConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAutoUpdateConfigCollection(config), nil
}

var autoUpdateVersion = resource{
	getHandler:    getAutoUpdateVersion,
	createHandler: createAutoUpdateVersion,
	updateHandler: updateAutoUpdateVersion,
	deleteHandler: deleteAutoUpdateVersion,
	singleton:     true,
}

func createAutoUpdateVersion(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

	if opts.force {
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

func updateAutoUpdateVersion(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func getAutoUpdateVersion(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	version, err := client.GetAutoUpdateVersion(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAutoUpdateVersionCollection(version), nil
}

func deleteAutoUpdateVersion(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteAutoUpdateVersion(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("AutoUpdateVersion has been deleted\n")
	return nil
}

var autoUpdateAgentRollout = resource{
	getHandler:    getAutoUpdateAgentRollout,
	createHandler: createAutoUpdateAgentRollout,
	updateHandler: updateAutoUpdateAgentRollout,
	deleteHandler: deleteAutoUpdateAgentRollout,
	singleton:     true,
}

func createAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

	if opts.force {
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

func getAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	version, err := client.GetAutoUpdateAgentRollout(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewAutoUpdateAgentRolloutCollection(version), nil
}

func updateAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteAutoUpdateAgentRollout(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.DeleteAutoUpdateAgentRollout(ctx); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("AutoUpdateAgentRollout has been deleted\n")
	return nil
}

var autoUpdateAgentReport = resource{
	getHandler:    getAutoUpdateAgentReport,
	createHandler: upsertAutoUpdateAgentReport,
}

func upsertAutoUpdateAgentReport(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func getAutoUpdateAgentReport(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		report, err := client.GetAutoUpdateAgentReport(ctx, ref.Name)
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
