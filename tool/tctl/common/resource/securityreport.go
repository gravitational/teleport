package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createAuditQuery(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalAuditQuery(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := in.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err = client.SecReportsClient().UpsertSecurityAuditQuery(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (rc *ResourceCommand) getAuditQuery(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		auditQuery, err := client.SecReportsClient().GetSecurityAuditQuery(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewAuditQueryCollection([]*secreports.AuditQuery{auditQuery}), nil
	}

	resources, err := client.SecReportsClient().GetSecurityAuditQueries(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return collections.NewAuditQueryCollection(resources), nil
}

func (rc *ResourceCommand) deleteAuditQuery(ctx context.Context, client *authclient.Client) error {
	if err := client.SecReportsClient().DeleteSecurityAuditQuery(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Audit query %q has been deleted\n", rc.ref.Name)
	return nil
}

func (rc *ResourceCommand) createSecurityReport(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalSecurityReport(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if err := in.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err = client.SecReportsClient().UpsertSecurityReport(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (rc *ResourceCommand) getSecurityReport(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {

		resource, err := client.SecReportsClient().GetSecurityReport(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewSecurityReportCollection([]*secreports.Report{resource}), nil
	}
	resources, err := client.SecReportsClient().GetSecurityReports(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return collections.NewSecurityReportCollection(resources), nil
}

func (rc *ResourceCommand) deleteSecurityReport(ctx context.Context, client *authclient.Client) error {
	if err := client.SecReportsClient().DeleteSecurityReport(ctx, rc.ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Security report %q has been deleted\n", rc.ref.Name)
	return nil
}
