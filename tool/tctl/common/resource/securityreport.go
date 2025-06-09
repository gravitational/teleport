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

var auditQuery = resource{
	getHandler:    getAuditQuery,
	createHandler: createAuditQuery,
	deleteHandler: deleteAuditQuery,
}

func createAuditQuery(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func getAuditQuery(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		auditQuery, err := client.SecReportsClient().GetSecurityAuditQuery(ctx, ref.Name)
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

func deleteAuditQuery(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.SecReportsClient().DeleteSecurityAuditQuery(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Audit query %q has been deleted\n", ref.Name)
	return nil
}

var securityReport = resource{
	getHandler:    getSecurityReport,
	createHandler: createSecurityReport,
	deleteHandler: deleteSecurityReport,
}

func createSecurityReport(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func getSecurityReport(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {

		resource, err := client.SecReportsClient().GetSecurityReport(ctx, ref.Name)
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

func deleteSecurityReport(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.SecReportsClient().DeleteSecurityReport(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Security report %q has been deleted\n", ref.Name)
	return nil
}
