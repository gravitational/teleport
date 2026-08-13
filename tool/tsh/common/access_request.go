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

package common

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/componentfeatures"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

var requestLoginHint = "use 'tsh login --request-id=<request-id>' to login with an approved request"

func onRequestList(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	cf.Username = cmp.Or(cf.Username, tc.Username)

	var reqs []types.AccessRequest

	// TODO: consider using the AccessRequestFilter below to filter server side rather than client side.
	err = tc.WithRootClusterClient(cf.Context, func(clt authclient.ClientI) error {
		reqs, err = clt.GetAccessRequests(cf.Context, types.AccessRequestFilter{})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// NOTE: It probably makes sense for --reviewable, --suggested, and --my-requests
	// to be mutually exclusive, but the original implementation of request filtering
	// applied the filters in this order. We retain that behavior now for compatibility.

	if cf.ReviewableRequests {
		reqs = slices.DeleteFunc(reqs, func(ar types.AccessRequest) bool {
			// Requests made by the same user or requests the user already reviewed are not reviewable.
			return ar.GetUser() == cf.Username ||
				slices.ContainsFunc(ar.GetReviews(), func(review types.AccessReview) bool { return review.Author == cf.Username })
		})
	}
	if cf.SuggestedRequests {
		reqs = slices.DeleteFunc(reqs, func(ar types.AccessRequest) bool {
			// Requests made by the same author, requests already reviewed, or requests that do not contain
			// this user as a suggested reviewer get filtered out.
			return ar.GetUser() == cf.Username ||
				slices.ContainsFunc(ar.GetReviews(), func(review types.AccessReview) bool { return review.Author == cf.Username }) ||
				!slices.ContainsFunc(ar.GetSuggestedReviewers(), func(suggestion string) bool { return suggestion == cf.Username })
		})
	}
	if cf.MyRequests {
		reqs = slices.DeleteFunc(reqs, func(ar types.AccessRequest) bool {
			// Filter out requests made by other users.
			return ar.GetUser() != cf.Username
		})
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		if err := showRequestTable(cf, reqs); err != nil {
			return trace.Wrap(err)
		}
	case teleport.JSON, teleport.YAML:
		out, err := serializeAccessRequests(reqs, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprint(cf.Stdout(), out)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
	return nil
}

func serializeAccessRequests(reqs []types.AccessRequest, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(reqs, "", "  ")
	} else {
		out, err = yaml.Marshal(reqs)
	}
	return string(out), trace.Wrap(err)
}

func onRequestShow(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if cf.Username == "" {
		cf.Username = tc.Username
	}

	var req types.AccessRequest
	err = tc.WithRootClusterClient(cf.Context, func(clt authclient.ClientI) error {
		req, err = services.GetAccessRequest(cf.Context, clt, cf.RequestID)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.Text, "":
		err = printRequest(cf, req)
		if err != nil {
			return trace.Wrap(err)
		}
	case teleport.JSON, teleport.YAML:
		out, err := serializeAccessRequest(req, format)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprint(cf.Stdout(), out)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
	return nil
}

func serializeAccessRequest(req types.AccessRequest, format string) (string, error) {
	var out []byte
	var err error
	if format == teleport.JSON {
		out, err = utils.FastMarshalIndent(req, "", "  ")
	} else {
		out, err = yaml.Marshal(req)
	}
	return string(out), trace.Wrap(err)
}

func printRequest(cf *CLIConf, req types.AccessRequest) error {
	reason := "[none]"
	if r := req.GetRequestReason(); r != "" {
		reason = fmt.Sprintf("%q", r)
	}

	reviewers := "[none]"
	if r := req.GetSuggestedReviewers(); len(r) > 0 {
		reviewers = strings.Join(r, ", ")
	}

	resourcesStr, err := common.FormatResourceAccessIDs(req.GetAllRequestedResourceIDs())
	if err != nil {
		return trace.Wrap(err)
	}

	table := asciitable.MakeHeadlessTable(2)
	table.AddRow([]string{"Request ID:", req.GetName()})
	table.AddRow([]string{"Username:", req.GetUser()})
	table.AddRow([]string{"Roles:", strings.Join(req.GetRoles(), ", ")})
	if len(resourcesStr) > 0 {
		table.AddRow([]string{"Resources:", resourcesStr})
	}
	table.AddRow([]string{"Reason:", reason})
	table.AddRow([]string{"Reviewers:", reviewers + " (suggested)"})
	if !req.GetAccessExpiry().IsZero() {
		// Display the expiry time in the local timezone. UTC is confusing.
		table.AddRow([]string{"Access Expires:", req.GetAccessExpiry().Local().Format(time.DateTime)})
	}
	if req.GetAssumeStartTime() != nil {
		table.AddRow([]string{"Assume Start Time:", req.GetAssumeStartTime().Local().Format(time.DateTime)})
	}
	table.AddRow([]string{"Status:", req.GetState().String()})

	_, err = table.AsBuffer().WriteTo(cf.Stdout())
	if err != nil {
		return trace.Wrap(err)
	}

	var approvals, denials []types.AccessReview

	for _, rev := range req.GetReviews() {
		switch {
		case rev.ProposedState.IsApproved():
			approvals = append(approvals, rev)
		case rev.ProposedState.IsDenied():
			denials = append(denials, rev)
		}
	}

	printReviewBlock := func(title string, revs []types.AccessReview) error {
		fmt.Fprintln(cf.Stdout(), "------------------------------------------------")
		fmt.Fprintf(cf.Stdout(), "%s:\n", title)

		for _, rev := range revs {
			fmt.Fprintln(cf.Stdout(), "  ----------------------------------------------")

			revReason := "[none]"
			if rev.Reason != "" {
				revReason = fmt.Sprintf("%q", rev.Reason)
			}

			subTable := asciitable.MakeHeadlessTable(2)
			subTable.AddRow([]string{"  Reviewer:", rev.Author})
			subTable.AddRow([]string{"  Reason:", revReason})
			_, err = subTable.AsBuffer().WriteTo(cf.Stdout())
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}

	if len(approvals) > 0 {
		if err := printReviewBlock("Approvals", approvals); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(denials) > 0 {
		if err := printReviewBlock("Denials", denials); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Fprintf(cf.Stdout(), "\nhint: %v\n", requestLoginHint)
	return nil
}

func onRequestCreate(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := executeAccessRequest(cf, tc); err != nil {
		return trace.Wrap(err)
	}

	onStatus(cf)
	return nil
}

func onRequestReview(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if cf.Username == "" {
		cf.Username = tc.Username
	}

	if cf.Approve == cf.Deny {
		return trace.BadParameter("must supply exactly one of '--approve' or '--deny'")
	}

	var parsedAssumeStartTime *time.Time
	if cf.AssumeStartTimeRaw != "" {
		assumeStartTime, err := time.Parse(time.RFC3339, cf.AssumeStartTimeRaw)
		if err != nil {
			return trace.BadParameter("parsing assume-start-time (required format RFC3339 e.g 2023-12-12T23:20:50.52Z): %v", err)
		}
		parsedAssumeStartTime = &assumeStartTime
	}

	var state types.RequestState
	switch {
	case cf.Approve:
		state = types.RequestState_APPROVED
	case cf.Deny:
		state = types.RequestState_DENIED
	}

	var req types.AccessRequest
	err = tc.WithRootClusterClient(cf.Context, func(clt authclient.ClientI) error {
		req, err = clt.SubmitAccessReview(cf.Context, types.AccessReviewSubmission{
			RequestID: cf.RequestID,
			Review: types.AccessReview{
				Author:          cf.Username,
				ProposedState:   state,
				Reason:          cf.ReviewReason,
				AssumeStartTime: parsedAssumeStartTime,
			},
		})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if s := req.GetState(); s.IsPending() || s == state {
		fmt.Fprintf(cf.Stderr(), "Successfully submitted review.  Request state: %s\n", req.GetState())
	} else {
		fmt.Fprintf(cf.Stderr(), "Warning: ineffectual review. Request state: %s\n", req.GetState())
	}
	return nil
}

func showRequestTable(cf *CLIConf, reqs []types.AccessRequest) error {
	sort.Slice(reqs, func(i, j int) bool {
		return reqs[i].GetCreationTime().After(reqs[j].GetCreationTime())
	})

	table := asciitable.MakeTable([]string{"ID", "User"})
	table.AddColumn(asciitable.Column{
		Title:         "Roles",
		MaxCellLength: 20,
		FootnoteLabel: "[+]",
	})
	table.AddColumn(asciitable.Column{
		Title:         "Resources",
		MaxCellLength: 20,
		FootnoteLabel: "[+]",
	})
	table.AddFootnote("[+]",
		"Columns are truncated, use 'tsh request show <request-id>' to view the full list")
	table.AddColumn(asciitable.Column{Title: "Created At (UTC)"})
	table.AddColumn(asciitable.Column{Title: "Request TTL"})
	table.AddColumn(asciitable.Column{Title: "Session TTL"})
	table.AddColumn(asciitable.Column{Title: "Assume Time (UTC)"})
	table.AddColumn(asciitable.Column{Title: "Status"})
	now := time.Now()
	for _, req := range reqs {
		if now.After(req.GetAccessExpiry()) {
			continue
		}
		// This table isn't a comprehensive overview of each request; omit constraints on resources for brevity
		// and only print their stringified ResourceIDs.
		resourceIDsString, err := types.ResourceIDsToString(types.RiskyExtractResourceIDs(req.GetAllRequestedResourceIDs()))
		if err != nil {
			return trace.Wrap(err)
		}
		assumeStartTime := ""
		if req.GetAssumeStartTime() != nil {
			assumeStartTime = req.GetAssumeStartTime().UTC().Format(time.RFC822)
		}
		table.AddRow([]string{
			req.GetName(),
			req.GetUser(),
			strings.Join(req.GetRoles(), ","),
			resourceIDsString,
			req.GetCreationTime().UTC().Format(time.RFC822),
			time.Until(req.Expiry()).Round(time.Minute).String(),
			time.Until(req.GetAccessExpiry()).Round(time.Minute).String(),
			assumeStartTime,
			req.GetState().String(),
		})
	}
	_, err := table.AsBuffer().WriteTo(cf.Stdout())

	fmt.Fprintf(cf.Stdout(), "\nhint: use 'tsh request show <request-id>' for additional details\n")
	fmt.Fprintf(cf.Stdout(), "      %v\n", requestLoginHint)
	return trace.Wrap(err)
}

func onRequestSearch(cf *CLIConf) error {
	if cf.RequestableRoles && cf.ResourceKind != "" {
		return trace.BadParameter("only one of --kind and --roles may be specified")
	}
	if !cf.RequestableRoles && cf.ResourceKind == "" {
		return trace.BadParameter("one of --kind and --roles is required")
	}

	if cf.RequestableRoles {
		return searchRequestableRoles(cf)
	} else {
		return searchRequestableResources(cf)
	}
}

type requestableRoleRow struct {
	Role        string
	Description string
}

type resourceRow interface {
	kubeResourceRow |
		dbResourceRow |
		genericResourceRow
}

type kubeResourceRow struct {
	Name       string
	Namespace  string
	Labels     string
	ResourceID string
}

type dbResourceRow struct {
	DatabaseName string
	Labels       string
	Access       string `json:"-"`
	ResourceID   string
	Principals   map[string]principalSplitJSON `json:",omitempty" asciitable:"-"`
}

type genericResourceRow struct {
	Name       string
	Hostname   string
	Labels     string
	Access     string `json:"-"`
	ResourceID string
	Principals map[string]principalSplitJSON `json:",omitempty" asciitable:"-"`
}

func searchRequestableRoles(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var allRoles []*proto.ListRequestableRolesResponse_RequestableRole
	err = tc.WithRootClusterClient(cf.Context, func(clt authclient.ClientI) error {
		pageFunc := func(ctx context.Context, pageSize int, pageToken string) ([]*proto.ListRequestableRolesResponse_RequestableRole, string, error) {
			req := proto.ListRequestableRolesRequest_builder{
				PageSize:  int32(pageSize),
				PageToken: pageToken,
			}.Build()

			resp, err := clt.ListRequestableRoles(ctx, req)
			return resp.GetRoles(), resp.GetNextPageToken(), trace.Wrap(err)
		}

		var err error
		allRoles, err = stream.Collect(clientutils.Resources(cf.Context, pageFunc))
		return err
	})
	if err != nil {
		return trace.Wrap(err)
	}

	rows := make([]requestableRoleRow, 0, len(allRoles))
	for _, r := range allRoles {
		rows = append(rows, requestableRoleRow{
			Role:        r.GetName(),
			Description: r.GetDescription(),
		})
	}

	return printRequestableRoles(cf, rows)
}

func printRequestableRoles(cf *CLIConf, rows []requestableRoleRow) error {
	format := strings.ToLower(cf.Format)

	switch format {
	case teleport.Text, "":
		if len(rows) == 0 {
			fmt.Fprintln(cf.Stdout(), "No requestable roles found.")
			return nil
		}

		columns, rows, err := asciitable.MakeColumnsAndRows(rows, nil)
		if err != nil {
			return err
		}

		var table asciitable.Table
		if cf.Verbose {
			table = asciitable.MakeTable(columns, rows...)
		} else {
			table = asciitable.MakeTableWithTruncatedColumn(columns, rows, "Description")
		}

		if _, err := table.AsBuffer().WriteTo(cf.Stdout()); err != nil {
			return trace.Wrap(err)
		}
		return nil

	case teleport.YAML:
		return trace.Wrap(utils.WriteYAMLArray(cf.Stdout(), rows))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(cf.Stdout(), rows))
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
}

// unifiedSearchKinds are the requestable resource kinds the unified resource
// cache serves (see the watch list in lib/services/unified_resource.go), and
// so the only kinds whose listings can carry per-dimension principal sets.
// Requestable kinds outside this set, such as user groups and Identity Center
// account assignments, are reachable only through ListResources; asking
// ListUnifiedResources for one returns an empty page rather than an error.
var unifiedSearchKinds = []string{
	types.KindNode,
	types.KindKubernetesCluster,
	types.KindDatabase,
	types.KindApp,
	types.KindWindowsDesktop,
	types.KindSAMLIdPServiceProvider,
	types.KindIdentityCenterAccount,
	types.KindGitServer,
}

// resourceSearchClient lists resources both ways, so a search can fall back to
// ListResources for kinds the unified resource cache does not serve.
type resourceSearchClient interface {
	client.ListResourcesClient
	client.ListUnifiedResourcesClient
}

// listRequestableResources lists the requestable resources of one kind.
// Kinds the unified resource cache serves go through ListUnifiedResources, so
// Auth returns each dimension's granted/requestable principal split instead of
// every client recomputing it. The remaining kinds keep the ListResources path
// and come back carrying no principal sets, which callers render the same way
// they render a cluster too old to populate them: an empty Access column, and
// no Principals key in JSON output.
func listRequestableResources(ctx context.Context, clt resourceSearchClient, req proto.ListResourcesRequest, kind string) ([]*types.EnrichedResource, error) {
	if !slices.Contains(unifiedSearchKinds, kind) {
		resources, err := accessrequest.GetResourcesByKind(ctx, clt, req, kind)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		enriched := make([]*types.EnrichedResource, 0, len(resources))
		for _, r := range resources {
			enriched = append(enriched, &types.EnrichedResource{ResourceWithLabels: r})
		}
		return enriched, nil
	}

	enriched, err := client.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
		Kinds:               []string{kind},
		Labels:              req.Labels,
		PredicateExpression: req.PredicateExpression,
		SearchKeywords:      req.SearchKeywords,
		UseSearchAsRoles:    req.UseSearchAsRoles,
		IncludeLogins:       true,
		IncludeRequestable:  true,
		SortBy:              types.SortBy{Field: types.ResourceKind},
	})
	return enriched, trace.Wrap(err)
}

func searchRequestableResources(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// If KubeCluster not provided try to read it from kubeconfig.
	if cf.KubernetesCluster == "" {
		cf.KubernetesCluster, _ = kubeconfig.SelectedKubeCluster(getKubeConfigPath(cf, ""), tc.SiteName)
	}
	if cf.KubernetesCluster == "" && cf.ResourceKind == types.KindKubernetesResource {
		return trace.BadParameter("--kube-cluster is required when searching for Kubernetes resources")
	}
	// if --all-namespaces flag was provided we search in every namespace.
	// This means sending an empty namespace to the ListResources API.
	if cf.kubeAllNamespaces {
		cf.kubeNamespace = ""
	}

	deduplicateResourceIDs := map[string]struct{}{}
	var resourceIDs []string

	switch cf.ResourceKind {
	case types.KindKubernetesResource:
		proxyGRPCClient, err := tc.NewKubernetesServiceClient(cf.Context, tc.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		resourceType := types.AccessRequestPrefixKindKube + cf.kubeResourceKind
		if cf.kubeAPIGroup != "" {
			resourceType = resourceType + "." + cf.kubeAPIGroup
		}
		req := kubeproto.ListKubernetesResourcesRequest_builder{
			ResourceType:        resourceType,
			Labels:              tc.Labels,
			PredicateExpression: cf.PredicateExpression,
			SearchKeywords:      tc.SearchKeywords,
			UseSearchAsRoles:    true,
			KubernetesCluster:   cf.KubernetesCluster,
			KubernetesNamespace: cf.kubeNamespace,
			TeleportCluster:     tc.SiteName,
		}.Build()

		resources, err := client.GetKubernetesResourcesWithFilters(cf.Context, proxyGRPCClient, req)
		if err != nil {
			return trace.Wrap(err)
		}

		var rows []kubeResourceRow
		for _, resource := range resources {
			r, ok := resource.(*types.KubernetesResourceV1)
			if !ok {
				continue
			}

			resourceID := types.ResourceIDToString(types.ResourceID{
				ClusterName:     tc.SiteName,
				Kind:            r.GetKind(),
				Name:            cf.KubernetesCluster,
				SubResourceName: path.Join(r.Spec.Namespace, r.GetName()),
			})
			if ignoreDuplicateResourceID(deduplicateResourceIDs, resourceID) {
				continue
			}
			resourceIDs = append(resourceIDs, resourceID)

			rows = append(rows, kubeResourceRow{
				Name:       common.FormatResourceName(r, cf.Verbose),
				Namespace:  r.Spec.Namespace,
				Labels:     common.FormatLabels(r.GetAllLabels(), cf.Verbose),
				ResourceID: resourceID,
			})
		}

		return printRequestableResources(cf, rows, resourceIDs)

	default:
		// For all other resources, we connect to the auth server and list
		// resources through whichever API serves this kind.
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		enriched, err := listRequestableResources(cf.Context, clusterClient.AuthClient, proto.ListResourcesRequest{
			Labels:              tc.Labels,
			PredicateExpression: cf.PredicateExpression,
			SearchKeywords:      tc.SearchKeywords,
			UseSearchAsRoles:    true,
		}, cf.ResourceKind)
		if err != nil {
			return trace.Wrap(err)
		}

		switch cf.ResourceKind {
		case types.KindDatabase:
			var rows []dbResourceRow
			for _, er := range enriched {
				leaf, err := accessrequest.MapListResourcesResultToLeafResource(er.ResourceWithLabels, cf.ResourceKind)
				if err != nil {
					return trace.Wrap(err)
				}

				resourceID := types.ResourceIDToString(types.ResourceID{
					ClusterName: tc.SiteName,
					Kind:        leaf.GetKind(),
					Name:        leaf.GetName(),
				})
				if ignoreDuplicateResourceID(deduplicateResourceIDs, resourceID) {
					continue
				}
				resourceIDs = append(resourceIDs, resourceID)

				splits := principalSplits(er)
				rows = append(rows, dbResourceRow{
					DatabaseName: common.FormatResourceName(leaf, cf.Verbose),
					Labels:       common.FormatLabels(leaf.GetAllLabels(), cf.Verbose),
					Access:       formatAccessSummary(splits),
					ResourceID:   resourceID,
					Principals:   principalSplitsJSON(splits),
				})
			}
			return printRequestableResources(cf, rows, resourceIDs)

		default:
			var rows []genericResourceRow
			for _, er := range enriched {
				leaf, err := accessrequest.MapListResourcesResultToLeafResource(er.ResourceWithLabels, cf.ResourceKind)
				if err != nil {
					return trace.Wrap(err)
				}

				resourceID := types.ResourceIDToString(types.ResourceID{
					ClusterName: tc.SiteName,
					Kind:        leaf.GetKind(),
					Name:        leaf.GetName(),
				})
				if ignoreDuplicateResourceID(deduplicateResourceIDs, resourceID) {
					continue
				}
				resourceIDs = append(resourceIDs, resourceID)

				hostName := ""
				if r2, ok := leaf.(interface{ GetHostname() string }); ok {
					hostName = r2.GetHostname()
				}

				splits := principalSplits(er)
				rows = append(rows, genericResourceRow{
					Name:       common.FormatResourceName(leaf, cf.Verbose),
					Hostname:   hostName,
					Labels:     common.FormatLabels(leaf.GetAllLabels(), cf.Verbose),
					Access:     formatAccessSummary(splits),
					ResourceID: resourceID,
					Principals: principalSplitsJSON(splits),
				})
			}

			return printRequestableResources(cf, rows, resourceIDs)
		}
	}
}

func printRequestableResources[T resourceRow](cf *CLIConf, rows []T, resourceIDs []string) error {
	format := strings.ToLower(cf.Format)

	switch format {
	case teleport.Text, "":
		columns, tableRows, err := asciitable.MakeColumnsAndRows(rows, nil)
		if err != nil {
			return err
		}

		var table asciitable.Table
		if cf.Verbose {
			table = asciitable.MakeTable(columns, tableRows...)
		} else {
			table = asciitable.MakeTableWithTruncatedColumn(columns, tableRows, "Labels")
		}

		if _, err := table.AsBuffer().WriteTo(cf.Stdout()); err != nil {
			return trace.Wrap(err)
		}

		if len(resourceIDs) > 0 {
			fmt.Fprint(cf.Stdout(), "\nhint: use 'tsh request show-principals <resource-id>' to view granted & requestable principals\n")

			resourcesStr := strings.Join(resourceIDs, " --resource ")
			fmt.Fprintf(cf.Stdout(), `
To request access to these resources, run
> tsh request create --resource %s \
    --reason <request reason>

`, resourcesStr)
		}

		return nil

	case teleport.YAML:
		return trace.Wrap(utils.WriteYAMLArray(cf.Stdout(), rows))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(cf.Stdout(), rows))
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
}

// ignoreDuplicateResourceID returns true if the resource ID is a duplicate
// and should be ignored. Otherwise, it returns false and adds the resource ID
// to the deduplicateResourceIDs map.
func ignoreDuplicateResourceID(deduplicateResourceIDs map[string]struct{}, resourceID string) bool {
	// Ignore duplicate resource IDs.
	if _, ok := deduplicateResourceIDs[resourceID]; ok {
		return true
	}
	deduplicateResourceIDs[resourceID] = struct{}{}
	return false
}

// principalSplit holds a resource's selectable principals divided into the set
// the user can already use (granted) and the set they must request
// (requestable).
type principalSplit struct {
	granted     []string
	requestable []string
}

// principalSplitJSON is one dimension's granted/requestable split in JSON
// output, keyed by the dimension's inline constraint key.
type principalSplitJSON struct {
	Granted     []string `json:"granted"`
	Requestable []string `json:"requestable"`
}

// principalSplitsJSON converts splits to their JSON form, with empty slices
// rather than nulls.
func principalSplitsJSON(splits map[string]principalSplit) map[string]principalSplitJSON {
	if len(splits) == 0 {
		return nil
	}
	out := make(map[string]principalSplitJSON, len(splits))
	for kind, s := range splits {
		j := principalSplitJSON{Granted: s.granted, Requestable: s.requestable}
		if j.Granted == nil {
			j.Granted = []string{}
		}
		if j.Requestable == nil {
			j.Requestable = []string{}
		}
		out[kind] = j
	}
	return out
}

// principalSplits derives each principal dimension's granted/requestable split
// from an enriched resource, keyed by the dimension's inline constraint key.
// When Auth did not populate principal sets (older version, or a kind without
// them), nil is returned rather than fabricating a split from the flat Logins
// union: JSON output omits the Principals map and text output leaves the
// Access cell empty.
func principalSplits(er *types.EnrichedResource) map[string]principalSplit {
	if len(er.Principals) == 0 {
		return nil
	}
	out := make(map[string]principalSplit, len(er.Principals))
	for _, ps := range er.Principals {
		granted := append([]string(nil), ps.Granted...)
		requestable := append([]string(nil), ps.Requestable...)
		sort.Strings(granted)
		sort.Strings(requestable)
		out[ps.PrincipalType] = principalSplit{granted: granted, requestable: requestable}
	}
	return out
}

// formatAccessSummary renders the compact "<n> granted, <m> requestable" cell
// for the search table, counting across every principal dimension and omitting
// either side when it is empty.
func formatAccessSummary(splits map[string]principalSplit) string {
	var granted, requestable int
	for _, s := range splits {
		granted += len(s.granted)
		requestable += len(s.requestable)
	}
	var parts []string
	if granted > 0 {
		parts = append(parts, fmt.Sprintf("%d granted", granted))
	}
	if requestable > 0 {
		parts = append(parts, fmt.Sprintf("%d requestable", requestable))
	}
	return strings.Join(parts, ", ")
}

// principalHeading is the human heading for a principal dimension in
// show-principals output. A dimension this build does not recognize (a newer cluster's) falls
// back to its raw key, which still names it usefully.
func principalHeading(kind string) string {
	switch kind {
	case types.PrincipalTypeLogins:
		return "Logins"
	case types.PrincipalTypeRoleARNs:
		return "Role ARNs"
	default:
		return kind
	}
}

// hintConstraintSuffix builds the inline constraint suffix for the create
// hint, covering every dimension with requestable values. A resource kind
// whose constraints nothing can enforce yet gets no suffix, keyed off the same
// map the Auth gate and the client pre-flight use, so a kind gains its hint at
// the moment it gains a feature ID. Values escape the characters the inline
// grammar treats specially: "\", ",", and "&".
func hintConstraintSuffix(resourceKind string, kinds []string, splits map[string]principalSplit) string {
	if _, ok := componentfeatures.ConstraintFeatureForKind(resourceKind); !ok {
		return ""
	}
	var b strings.Builder
	for _, kind := range kinds {
		s := splits[kind]
		if len(s.requestable) == 0 {
			continue
		}
		escaped := make([]string, 0, len(s.requestable))
		for _, v := range s.requestable {
			escaped = append(escaped, common.EscapeConstraintValue(v))
		}
		if b.Len() == 0 {
			b.WriteString("?")
		} else {
			b.WriteString("&")
		}
		fmt.Fprintf(&b, "%s=%s", kind, strings.Join(escaped, ","))
	}
	return b.String()
}

func hostnameOf(r types.ResourceWithLabels) string {
	if h, ok := r.(interface{ GetHostname() string }); ok {
		return h.GetHostname()
	}
	return ""
}

// resourcePrincipalsJSON is the structured output of `tsh request
// show-principals --format json`: the full granted and requestable split for
// every principal dimension of a single resource, keyed by the dimension's
// inline constraint key, so an agent can construct a constrained request in
// one call.
type resourcePrincipalsJSON struct {
	ResourceID string                        `json:"resource_id"`
	Kind       string                        `json:"kind"`
	Name       string                        `json:"name"`
	Hostname   string                        `json:"hostname,omitempty"`
	Labels     map[string]string             `json:"labels,omitempty"`
	Principals map[string]principalSplitJSON `json:"principals"`
}

// createdRequestJSON is the structured output of `tsh request create --format
// json`, carrying what an agent needs to track the request it just made: the
// id to poll or assume, the state it landed in, the roles it resolved to, and
// the resources it covers in the same inline form --resource accepts.
type createdRequestJSON struct {
	ID        string   `json:"id"`
	State     string   `json:"state"`
	User      string   `json:"user"`
	Roles     []string `json:"roles"`
	Resources []string `json:"resources,omitempty"`
}

// structuredRequestOutput reports whether the user asked for machine-readable
// output, in which case human progress messages move off stdout.
func structuredRequestOutput(cf *CLIConf) bool {
	switch strings.ToLower(cf.Format) {
	case teleport.JSON, teleport.YAML:
		return true
	default:
		return false
	}
}

// requestProgressWriter returns the stream human progress messages belong on:
// stdout normally, stderr when stdout is carrying a structured document.
func requestProgressWriter(cf *CLIConf) io.Writer {
	if structuredRequestOutput(cf) {
		return cf.Stderr()
	}
	return cf.Stdout()
}

// printCreatedRequest writes the created request as a single JSON or YAML
// document.
func printCreatedRequest(cf *CLIConf, req types.AccessRequest) error {
	payload := createdRequestJSON{
		ID:    req.GetName(),
		State: req.GetState().String(),
		User:  req.GetUser(),
		Roles: req.GetRoles(),
	}
	if payload.Roles == nil {
		payload.Roles = []string{}
	}
	for _, raid := range req.GetRequestedResourceAccessIDs() {
		// Inline form, not the display form: an agent reading this is expected
		// to feed the value straight back to --resource.
		payload.Resources = append(payload.Resources, common.FormatResourceAccessIDInline(raid))
	}

	if strings.ToLower(cf.Format) == teleport.YAML {
		return trace.Wrap(utils.WriteYAML(cf.Stdout(), payload))
	}
	return trace.Wrap(utils.WriteJSON(cf.Stdout(), payload))
}

// principalsTargetCluster returns the cluster whose presence serves a
// show-principals query: the ResourceID's cluster when set, else the connected
// cluster.
func principalsTargetCluster(currentCluster string, id types.ResourceID) string {
	if id.ClusterName != "" {
		return id.ClusterName
	}
	return currentCluster
}

// onRequestShowPrincipals shows the granted vs. requestable principals for a
// single resource, identified by its resource ID (e.g. /cluster/node/web-1),
// so a user or agent can decide which principals to scope a request to.
func onRequestShowPrincipals(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	id, err := types.ResourceIDFromString(cf.RequestShowPrincipalsResourceID)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	// Route the query to the resource's own cluster: principals of a leaf
	// resource must be read from the leaf's presence, not the connected
	// cluster's.
	authClient := clusterClient.CurrentCluster()
	if target := principalsTargetCluster(clusterClient.ClusterName(), id); target != clusterClient.ClusterName() {
		leafClient, err := clusterClient.ConnectToCluster(cf.Context, target)
		if err != nil {
			return trace.Wrap(err)
		}
		defer leafClient.Close()
		authClient = leafClient
	}

	enriched, err := client.GetAllUnifiedResources(cf.Context, authClient, &proto.ListUnifiedResourcesRequest{
		Kinds:               []string{id.Kind},
		PredicateExpression: fmt.Sprintf(`name == %q`, id.Name),
		UseSearchAsRoles:    true,
		IncludeLogins:       true,
		IncludeRequestable:  true,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, er := range enriched {
		leaf, err := accessrequest.MapListResourcesResultToLeafResource(er.ResourceWithLabels, id.Kind)
		if err != nil {
			continue
		}
		if leaf.GetName() == id.Name {
			return trace.Wrap(printResourcePrincipals(cf, id, leaf, principalSplits(er)))
		}
	}
	return trace.NotFound("resource %q was not found or is not requestable", cf.RequestShowPrincipalsResourceID)
}

func printResourcePrincipals(cf *CLIConf, id types.ResourceID, leaf types.ResourceWithLabels, splits map[string]principalSplit) error {
	idStr := types.ResourceIDToString(id)

	switch strings.ToLower(cf.Format) {
	case teleport.JSON, teleport.YAML:
		payload := resourcePrincipalsJSON{
			ResourceID: idStr,
			Kind:       id.Kind,
			Name:       leaf.GetName(),
			Hostname:   hostnameOf(leaf),
			Labels:     leaf.GetAllLabels(),
			Principals: principalSplitsJSON(splits),
		}
		if payload.Principals == nil {
			payload.Principals = map[string]principalSplitJSON{}
		}
		if strings.ToLower(cf.Format) == teleport.JSON {
			return trace.Wrap(utils.WriteJSON(cf.Stdout(), payload))
		}
		return trace.Wrap(utils.WriteYAML(cf.Stdout(), payload))
	case teleport.Text, "":
		// handled below
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}

	w := cf.Stdout()
	fmt.Fprintf(w, "Resource:  %s\n", idStr)
	fmt.Fprintf(w, "Name:      %s\n", leaf.GetName())
	if h := hostnameOf(leaf); h != "" {
		fmt.Fprintf(w, "Hostname:  %s\n", h)
	}
	if labels := common.FormatLabels(leaf.GetAllLabels(), cf.Verbose); labels != "" {
		fmt.Fprintf(w, "Labels:    %s\n", labels)
	}

	if len(splits) == 0 {
		fmt.Fprint(w, "\nNo selectable principals for this resource.\n")
		return nil
	}

	kinds := make([]string, 0, len(splits))
	for kind := range splits {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)

	for _, kind := range kinds {
		s := splits[kind]
		if len(s.granted) == 0 && len(s.requestable) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s:\n", principalHeading(kind))
		for _, p := range s.granted {
			fmt.Fprintf(w, "  %-20s granted\n", p)
		}
		for _, p := range s.requestable {
			fmt.Fprintf(w, "  %-20s requestable\n", p)
		}
	}

	if suffix := hintConstraintSuffix(id.Kind, kinds, splits); suffix != "" {
		fmt.Fprintf(w, "\nhint: tsh request create --resource '%s%s' --reason \"...\"\n", idStr, suffix)
	} else {
		fmt.Fprintf(w, "\nhint: tsh request create --resource '%s' --reason \"...\"\n", idStr)
	}
	return nil
}

func onRequestDrop(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(cf.RequestIDs) == 1 && cf.RequestIDs[0] == "*" {
		fmt.Fprintf(cf.Stdout(), "Dropping all active access requests...\n\n")
	} else {
		fmt.Fprintf(cf.Stdout(), "Dropping access request(s): %s...\n\n", strings.Join(cf.RequestIDs, ", "))
	}
	if err := reissueWithRequests(cf, tc, nil /*newRequests*/, cf.RequestIDs); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(onStatus(cf))
}
