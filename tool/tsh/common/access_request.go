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
	Access       string                        `json:"-"`
	ResourceID   string                        `asciitable:"-"`
	Principals   map[string]principalSplitJSON `json:",omitempty" asciitable:"-"`
}

type genericResourceRow struct {
	Name       string
	Hostname   string
	Labels     string
	Access     string                        `json:"-"`
	ResourceID string                        `asciitable:"-"`
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
		// unified resources so Auth returns the granted/requestable split per
		// principal dimension rather than each client recomputing it.
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		enriched, err := client.GetAllUnifiedResources(cf.Context, clusterClient.AuthClient, &proto.ListUnifiedResourcesRequest{
			Kinds:               []string{cf.ResourceKind},
			Labels:              tc.Labels,
			PredicateExpression: cf.PredicateExpression,
			SearchKeywords:      tc.SearchKeywords,
			UseSearchAsRoles:    true,
			IncludeLogins:       true,
			IncludeRequestable:  true,
			SortBy:              types.SortBy{Field: types.ResourceKind},
		})
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

				splits := principalSplits(er, cf.ResourceKind)
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

				splits := principalSplits(er, cf.ResourceKind)
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
			fmt.Fprint(cf.Stdout(), "\nhint: use 'tsh request preview <resource-id>' to view granted & requestable principals\n")

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
// Auth's principal sets are preferred (requestable is All minus Granted). When
// Auth did not populate them (older version), the flat Logins union is used
// with every value treated as requestable, matching the mixed-version fallback
// of showing the union without the distinction.
func principalSplits(er *types.EnrichedResource, resourceKind string) map[string]principalSplit {
	if len(er.Principals) > 0 {
		out := make(map[string]principalSplit, len(er.Principals))
		for _, ps := range er.Principals {
			grantedSet := make(map[string]struct{}, len(ps.Granted))
			for _, g := range ps.Granted {
				grantedSet[g] = struct{}{}
			}
			granted := append([]string(nil), ps.Granted...)
			var requestable []string
			for _, v := range ps.All {
				if _, ok := grantedSet[v]; !ok {
					requestable = append(requestable, v)
				}
			}
			sort.Strings(granted)
			sort.Strings(requestable)
			out[ps.Kind] = principalSplit{granted: granted, requestable: requestable}
		}
		return out
	}
	if len(er.Logins) == 0 {
		return nil
	}
	requestable := append([]string(nil), er.Logins...)
	sort.Strings(requestable)
	return map[string]principalSplit{
		fallbackPrincipalKind(resourceKind): {requestable: requestable},
	}
}

// fallbackPrincipalKind names the dimension the flat Logins union carries for
// a resource kind, for listings from an Auth that predates principal sets.
func fallbackPrincipalKind(resourceKind string) string {
	if resourceKind == types.KindApp {
		return types.PrincipalKindRoleARNs
	}
	return types.PrincipalKindLogins
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

// principalHeading is the human heading for a principal dimension in preview
// output. A dimension this build does not recognize (a newer cluster's) falls
// back to its raw key, which still names it usefully.
func principalHeading(kind string) string {
	switch kind {
	case types.PrincipalKindLogins:
		return "Logins"
	case types.PrincipalKindRoleARNs:
		return "Role ARNs"
	default:
		return kind
	}
}

// hintConstraintSuffix builds the inline constraint suffix for the create
// hint, covering every dimension with requestable values. Kinds without an
// enforceable constraint yet (desktops, for now) get no suffix.
func hintConstraintSuffix(resourceKind string, kinds []string, splits map[string]principalSplit) string {
	if resourceKind != types.KindNode && resourceKind != types.KindApp {
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
			v = strings.ReplaceAll(v, `\`, `\\`)
			v = strings.ReplaceAll(v, ",", `\,`)
			escaped = append(escaped, v)
		}
		fmt.Fprintf(&b, "|%s=%s", kind, strings.Join(escaped, ","))
	}
	return b.String()
}

func hostnameOf(r types.ResourceWithLabels) string {
	if h, ok := r.(interface{ GetHostname() string }); ok {
		return h.GetHostname()
	}
	return ""
}

// resourcePreviewJSON is the structured output of `tsh request preview
// --format json`: the full granted and requestable split for every principal
// dimension of a single resource, keyed by the dimension's inline constraint
// key, so an agent can construct a constrained request in one call.
type resourcePreviewJSON struct {
	ResourceID string                        `json:"resource_id"`
	Kind       string                        `json:"kind"`
	Name       string                        `json:"name"`
	Hostname   string                        `json:"hostname,omitempty"`
	Labels     map[string]string             `json:"labels,omitempty"`
	Principals map[string]principalSplitJSON `json:"principals"`
}

// onRequestPreview shows the granted vs. requestable principals for a single
// resource, identified by its resource ID (e.g. /cluster/node/web-1), so a user
// or agent can decide which principals to scope a request to.
func onRequestPreview(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	id, err := types.ResourceIDFromString(cf.RequestPreviewResourceID)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	enriched, err := client.GetAllUnifiedResources(cf.Context, clusterClient.AuthClient, &proto.ListUnifiedResourcesRequest{
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
			return trace.Wrap(printResourcePreview(cf, id, leaf, principalSplits(er, id.Kind)))
		}
	}
	return trace.NotFound("resource %q was not found or is not requestable", cf.RequestPreviewResourceID)
}

func printResourcePreview(cf *CLIConf, id types.ResourceID, leaf types.ResourceWithLabels, splits map[string]principalSplit) error {
	idStr := types.ResourceIDToString(id)

	switch strings.ToLower(cf.Format) {
	case teleport.JSON, teleport.YAML:
		payload := resourcePreviewJSON{
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
