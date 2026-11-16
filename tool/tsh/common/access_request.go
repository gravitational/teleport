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
	"maps"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"golang.org/x/term"

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
	timing := req.GetTiming()
	if timing != nil {
		scheduled := timing.GetScheduled()
		if scheduled == nil {
			return trace.BadParameter("invalid timing parameters: missing scheduled mode")
		}

		end := scheduled.Start.Add(scheduled.Duration)
		start := scheduled.Start
		now := time.Now()
		if now.After(start) && now.Before(end) {
			start = now
		}

		table.AddRow([]string{"Access Starts:", timeZoneFromEnvOrLocal(start).Format(time.RFC1123)})
		table.AddRow([]string{"Access Ends:", timeZoneFromEnvOrLocal(end).Format(time.RFC1123)})
		table.AddRow([]string{"Access Duration:", end.Sub(start).String()})
		if req.GetState().IsPending() {
			table.AddRow([]string{"Approval Required By:", timeZoneFromEnvOrLocal(req.Expiry()).Format(time.RFC1123)})
		}

	} else {
		if !req.GetAccessExpiry().IsZero() {
			table.AddRow([]string{"Access Expires:", timeZoneFromEnvOrLocal(req.GetAccessExpiry()).Format(time.RFC1123)})
		}
		if req.GetAssumeStartTime() != nil {
			table.AddRow([]string{"Assume Start Time:", timeZoneFromEnvOrLocal(*req.GetAssumeStartTime()).Format(time.RFC1123)})
		}
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

	// onStatus also reads --format, and prints a second document
	if !structuredRequestOutput(cf) {
		onStatus(cf)
	}
	return nil
}

// parseGrantedTraits parses the --grant-trait flag values, each of which
// must be of the form "key=value1,value2,...". Each key may only be
// specified once across all occurrences of the flag.
func parseGrantedTraits(raw []string) ([]*types.GrantedTrait, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(raw))
	traits := make([]*types.GrantedTrait, 0, len(raw))
	for _, entry := range raw {
		key, rawValues, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, trace.BadParameter("invalid --grant-trait %q, expected format key=value1,value2", entry)
		}

		if _, exists := seen[key]; exists {
			return nil, trace.BadParameter("trait %q specified multiple times in --grant-trait", key)
		}

		var values []string
		for _, v := range strings.Split(rawValues, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			values = append(values, v)
		}
		if len(values) == 0 {
			return nil, trace.BadParameter("invalid --grant-trait %q, expected at least one value for trait %q", entry, key)
		}

		seen[key] = struct{}{}
		traits = append(traits, &types.GrantedTrait{Name: key, Values: values})
	}

	return traits, nil
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

	if len(cf.GrantTraits) > 0 && cf.Deny {
		return trace.BadParameter("--grant-trait can only be used with --approve")
	}

	grantedTraits, err := parseGrantedTraits(cf.GrantTraits)
	if err != nil {
		return trace.Wrap(err)
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
				GrantedTraits:   grantedTraits,
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
	table.AddColumn(asciitable.Column{Title: "Created At"})
	table.AddColumn(asciitable.Column{Title: "Access Starts"})
	table.AddColumn(asciitable.Column{Title: "Access Duration"})
	table.AddColumn(asciitable.Column{Title: "Access Ends"})
	table.AddColumn(asciitable.Column{Title: "Approve By"})
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

		scheduled := req.GetTiming().GetScheduled()

		accessStarts := req.GetCreationTime()
		if scheduled != nil {
			accessStarts = scheduled.Start
		} else if req.GetAssumeStartTime() != nil {
			accessStarts = *req.GetAssumeStartTime()
		}

		accessEnds := req.GetAccessExpiry()
		if scheduled != nil {
			accessEnds = scheduled.Start.Add(scheduled.Duration)
		}

		table.AddRow([]string{
			req.GetName(),
			req.GetUser(),
			strings.Join(req.GetRoles(), ","),
			resourceIDsString,
			timeZoneFromEnvOrLocal(req.GetCreationTime()).Format(time.RFC822),
			timeZoneFromEnvOrLocal(accessStarts).Format(time.RFC822),
			accessEnds.Sub(accessStarts).String(),
			timeZoneFromEnvOrLocal(accessEnds).Format(time.RFC822),
			timeZoneFromEnvOrLocal(req.Expiry()).Format(time.RFC822),
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

var principalResourceKinds = []string{
	types.KindApp,
	types.KindDatabase,
	types.KindGitServer,
	types.KindKubernetesCluster,
	types.KindLinuxDesktop,
	types.KindNode,
	types.KindSAMLIdPServiceProvider,
	types.KindWindowsDesktop,
}

type resourceSearchClient interface {
	client.ListResourcesClient
	client.ListUnifiedResourcesClient
}

// listRequestableResources uses ListUnifiedResources where possible. Only that
// listing reports principals.
func listRequestableResources(ctx context.Context, clt resourceSearchClient, req proto.ListResourcesRequest, kind string) ([]*types.EnrichedResource, error) {
	// Unified listing filters SAML providers as apps, rejecting their own kind.
	if slices.Contains(principalResourceKinds, kind) && kind != types.KindSAMLIdPServiceProvider {
		enriched, err := client.GetAllUnifiedResources(ctx, clt, &proto.ListUnifiedResourcesRequest{
			Kinds:               []string{kind},
			Labels:              req.Labels,
			PredicateExpression: req.PredicateExpression,
			SearchKeywords:      req.SearchKeywords,
			UseSearchAsRoles:    req.UseSearchAsRoles,
			IncludeLogins:       true,
			IncludeRequestable:  true,
		})
		if err == nil {
			if kind == types.KindApp {
				// Synthetic Identity Center apps require aws_ic_account IDs.
				enriched = slices.DeleteFunc(enriched, func(resource *types.EnrichedResource) bool {
					return resource.GetSubKind() == types.KindIdentityCenterAccount
				})
			}
			return enriched, nil
		}
		// Unsupported kind. Current Auth returns access denied, older Auth bad parameter.
		if !trace.IsAccessDenied(err) && !trace.IsBadParameter(err) {
			return nil, trace.Wrap(err)
		}
	}

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

		// Kube service listings include no principals.
		return printRequestableResources(cf, rows, resourceIDs, false)

	default:
		// For all other resources, we need to connect to the auth server.
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

		var dbRows []dbResourceRow
		var rows []genericResourceRow
		var anyPrincipals bool
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
			anyPrincipals = anyPrincipals || len(splits) > 0
			if cf.ResourceKind == types.KindDatabase {
				dbRows = append(dbRows, dbResourceRow{
					DatabaseName: common.FormatResourceName(leaf, cf.Verbose),
					Labels:       common.FormatLabels(leaf.GetAllLabels(), cf.Verbose),
					Access:       formatAccessSummary(splits),
					ResourceID:   resourceID,
					Principals:   principalSplitsJSON(splits),
				})
			} else {
				rows = append(rows, genericResourceRow{
					Name:       common.FormatResourceName(leaf, cf.Verbose),
					Hostname:   hostnameOf(leaf),
					Labels:     common.FormatLabels(leaf.GetAllLabels(), cf.Verbose),
					Access:     formatAccessSummary(splits),
					ResourceID: resourceID,
					Principals: principalSplitsJSON(splits),
				})
			}
		}
		if cf.ResourceKind == types.KindDatabase {
			return printRequestableResources(cf, dbRows, resourceIDs, anyPrincipals)
		}
		return printRequestableResources(cf, rows, resourceIDs, anyPrincipals)
	}
}

func printRequestableResources[T resourceRow](cf *CLIConf, rows []T, resourceIDs []string, anyPrincipals bool) error {
	format := strings.ToLower(cf.Format)

	switch format {
	case teleport.Text, "":
		columns, tableRows, err := asciitable.MakeColumnsAndRows(rows, nil)
		if err != nil {
			return err
		}
		table := asciitable.MakeHeadlessTable(0)
		for _, title := range columns {
			column := asciitable.Column{Title: title}
			switch title {
			case "Labels":
				column.Priority, column.MinWidth = 1, 16
			case "Access":
				column.Priority = 2
				column.OmitTruncated = true
				column.Hidden = !anyPrincipals
			}
			table.AddColumn(column)
		}
		for _, row := range tableRows {
			table.AddRow(row)
		}
		if !cf.Verbose {
			table.FitToWidth(terminalWidth(cf.Stdout()))
		}

		if _, err := table.AsBuffer().WriteTo(cf.Stdout()); err != nil {
			return trace.Wrap(err)
		}
		if len(resourceIDs) > 0 {
			if anyPrincipals {
				fmt.Fprint(cf.Stdout(), "\nhint: 'tsh request show-principals <resource-id>' shows a resource's granted and requestable principals.\n")
			}

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

// terminalWidth returns out's terminal width, or 80 if it cannot be determined.
func terminalWidth(out io.Writer) int {
	if file, ok := out.(*os.File); ok {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return 80
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

type principalSplit struct {
	granted     []string
	requestable []string
	// byRole maps roles to values they grant. Auth sets it only for kinds whose
	// dimensions must be granted by a single role.
	byRole []types.RolePrincipalValues
}

type principalSplitJSON struct {
	Granted     []string `json:"granted"`
	Requestable []string `json:"requestable"`
}

// principalSplitsJSON converts splits for output, with [] instead of null for empty lists.
func principalSplitsJSON(splits map[string]principalSplit) map[string]principalSplitJSON {
	if len(splits) == 0 {
		return nil
	}
	out := make(map[string]principalSplitJSON, len(splits))
	for principalType, s := range splits {
		j := principalSplitJSON{Granted: s.granted, Requestable: s.requestable}
		if j.Granted == nil {
			j.Granted = []string{}
		}
		if j.Requestable == nil {
			j.Requestable = []string{}
		}
		out[principalType] = j
	}
	return out
}

// principalSplits indexes er.Principals by constraint key. nil for a flat login list.
func principalSplits(er *types.EnrichedResource) map[string]principalSplit {
	if len(er.Principals) == 0 {
		return nil
	}
	out := make(map[string]principalSplit, len(er.Principals))
	for _, ps := range er.Principals {
		out[ps.PrincipalType] = principalSplit{
			granted:     slices.Sorted(slices.Values(ps.Granted)),
			requestable: slices.Sorted(slices.Values(ps.Requestable)),
			byRole:      ps.ByRole,
		}
	}
	return out
}

// formatAccessSummary counts granted and requestable values per dimension.
func formatAccessSummary(splits map[string]principalSplit) string {
	var parts []string
	for _, principalType := range slices.Sorted(maps.Keys(splits)) {
		s := splits[principalType]
		if len(s.granted) == 0 && len(s.requestable) == 0 {
			continue
		}
		summary := fmt.Sprintf("%d granted", len(s.granted))
		if len(s.requestable) > 0 {
			summary += fmt.Sprintf(" (+%d)", len(s.requestable))
		}
		if len(splits) > 1 {
			summary = principalType + " " + summary
		}
		parts = append(parts, summary)
	}
	return strings.Join(parts, "; ")
}

// requestableLiterals drops "*", which the constraint parser rejects.
func requestableLiterals(s principalSplit) []string {
	return slices.DeleteFunc(slices.Clone(s.requestable), func(v string) bool { return v == types.Wildcard })
}

// roleRequestable returns the sorted requestable literals of one dimension that role grants.
func roleRequestable(s principalSplit, role string) []string {
	literals := requestableLiterals(s)
	var keep []string
	for _, br := range s.byRole {
		if br.Role != role {
			continue
		}
		if slices.Contains(br.Values, types.Wildcard) {
			// "*" equals "any value"; every literal counts
			keep = literals
			break
		}
		for _, v := range br.Values {
			if slices.Contains(literals, v) && !slices.Contains(keep, v) {
				keep = append(keep, v)
			}
		}
	}
	slices.Sort(keep)
	return keep
}

// Combining dimensions from different roles can produce unusable hints.
// Omit requestable values unless one role grants every attributed dimension.
func coGrantedRequestable(splits map[string]principalSplit) map[string]principalSplit {
	var roles []string
	for _, s := range splits {
		for _, br := range s.byRole {
			if !slices.Contains(roles, br.Role) {
				roles = append(roles, br.Role)
			}
		}
	}
	if len(roles) == 0 {
		return splits
	}
	slices.Sort(roles)

	bestCount := 0
	var bestValues map[string][]string
roleSelection:
	for _, role := range roles {
		values := make(map[string][]string, len(splits))
		count := 0
		for principalType, s := range splits {
			if len(s.byRole) > 0 && !slices.ContainsFunc(s.byRole, func(br types.RolePrincipalValues) bool {
				return br.Role == role && len(br.Values) > 0
			}) {
				continue roleSelection
			}
			keep := roleRequestable(s, role)
			values[principalType] = keep
			count += len(keep)
		}
		if bestValues == nil || count > bestCount {
			bestCount, bestValues = count, values
		}
	}
	out := make(map[string]principalSplit, len(splits))
	for principalType, s := range splits {
		if bestValues == nil || len(s.byRole) > 0 {
			s.requestable = bestValues[principalType]
		}
		out[principalType] = s
	}
	return out
}

// hintConstraintSuffix returns inline constraints for requestable principals.
// Dimensions without explicit values or supported constraint keys are omitted.
func hintConstraintSuffix(resourceKind string, principalTypes []string, splits map[string]principalSplit) string {
	splits = coGrantedRequestable(splits)
	var variant string
	var pairs []string
	for _, principalType := range principalTypes {
		keyVariant, ok := common.ConstraintKeyVariant(resourceKind, principalType)
		if !ok || (variant != "" && keyVariant != variant) {
			continue
		}
		literals := requestableLiterals(splits[principalType])
		if len(literals) == 0 {
			continue
		}
		escaped := make([]string, 0, len(literals))
		for _, v := range literals {
			escaped = append(escaped, common.EscapeConstraintValue(v))
		}
		variant = keyVariant
		pairs = append(pairs, principalType+"="+strings.Join(escaped, ","))
	}
	if len(pairs) == 0 {
		return ""
	}
	return "?" + strings.Join(pairs, "&")
}

func hostnameOf(r types.ResourceWithLabels) string {
	if h, ok := r.(interface{ GetHostname() string }); ok {
		return h.GetHostname()
	}
	return ""
}

// resourcePrincipalsJSON is the structured output of `tsh request show-principals`.
type resourcePrincipalsJSON struct {
	ResourceID string            `json:"resource_id"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Hostname   string            `json:"hostname,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	// Principals is keyed by inline constraint key.
	Principals map[string]principalSplitJSON `json:"principals"`
	// Logins is the flat login list for kinds Auth reports without split principals.
	Logins []string `json:"logins,omitempty"`
}

// createdRequestJSON is the structured output of `tsh request create`.
type createdRequestJSON struct {
	ID    string   `json:"id"`
	State string   `json:"state"`
	User  string   `json:"user"`
	Roles []string `json:"roles"`
	// Resources are in --resource flag inline form.
	Resources []string `json:"resources"`
}

func structuredRequestOutput(cf *CLIConf) bool {
	switch strings.ToLower(cf.Format) {
	case teleport.JSON, teleport.YAML:
		return true
	default:
		return false
	}
}

func requestProgressWriter(cf *CLIConf) io.Writer {
	if structuredRequestOutput(cf) {
		return cf.Stderr()
	}
	return cf.Stdout()
}

func printCreatedRequest(cf *CLIConf, req types.AccessRequest) error {
	payload := createdRequestJSON{
		ID:        req.GetName(),
		State:     req.GetState().String(),
		User:      req.GetUser(),
		Roles:     []string{},
		Resources: []string{},
	}
	payload.Roles = append(payload.Roles, req.GetRoles()...)
	for _, raid := range req.GetAllRequestedResourceIDs() {
		payload.Resources = append(payload.Resources, common.FormatResourceAccessIDInline(raid))
	}

	if strings.ToLower(cf.Format) == teleport.YAML {
		return trace.Wrap(utils.WriteYAML(cf.Stdout(), payload))
	}
	return trace.Wrap(utils.WriteJSON(cf.Stdout(), payload))
}

func onRequestShowPrincipals(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	id, err := types.ResourceIDFromString(cf.RequestShowPrincipalsResourceID)
	if err != nil {
		return trace.Wrap(err)
	}
	if !slices.Contains(principalResourceKinds, id.Kind) {
		return trace.BadParameter("cannot show principals for resource %q. Use a resource with one of these kinds: %s",
			cf.RequestShowPrincipalsResourceID, strings.Join(principalResourceKinds, ", "))
	}

	clusterClient, err := tc.ConnectToCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	// Leaf resources are only in the leaf's cache
	authClient, err := clusterClient.ConnectToCluster(cf.Context, id.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer authClient.Close()

	enriched, err := listRequestableResources(cf.Context, authClient, proto.ListResourcesRequest{
		// Bare "name" in a predicate means a node's hostname rather than its metadata.name
		PredicateExpression: fmt.Sprintf("resource.metadata.name == %q", id.Name),
		UseSearchAsRoles:    true,
	}, id.Kind)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, er := range enriched {
		leaf, err := accessrequest.MapListResourcesResultToLeafResource(er.ResourceWithLabels, id.Kind)
		if err != nil {
			return trace.Wrap(err)
		}
		if leaf.GetName() == id.Name {
			return trace.Wrap(printResourcePrincipals(cf, id, leaf, principalSplits(er), er.Logins))
		}
	}
	var nodeHint string
	if id.Kind == types.KindNode {
		nodeHint = " Use the node UUID in the resource ID."
	}
	searchCommand := fmt.Sprintf("TELEPORT_CLUSTER='%s' tsh request search --kind %s", strings.ReplaceAll(id.ClusterName, "'", `'\''`), id.Kind)
	return trace.NotFound("resource %q not found or not requestable.%s Run %s to list requestable resources",
		cf.RequestShowPrincipalsResourceID, nodeHint, searchCommand)
}

// printResourcePrincipals prints splits, or logins when there is no split.
func printResourcePrincipals(cf *CLIConf, id types.ResourceID, leaf types.ResourceWithLabels, splits map[string]principalSplit, logins []string) error {
	idStr := types.ResourceIDToString(id)
	quotedID := "'" + strings.ReplaceAll(idStr, "'", `'\''`) + "'"
	if len(splits) > 0 {
		logins = nil
	}
	logins = slices.Sorted(slices.Values(logins))

	format := strings.ToLower(cf.Format)
	switch format {
	case teleport.JSON, teleport.YAML:
		payload := resourcePrincipalsJSON{
			ResourceID: idStr,
			Kind:       id.Kind,
			Name:       leaf.GetName(),
			Hostname:   hostnameOf(leaf),
			Labels:     leaf.GetAllLabels(),
			Principals: principalSplitsJSON(splits),
			Logins:     logins,
		}
		if payload.Principals == nil {
			payload.Principals = map[string]principalSplitJSON{}
		}
		if format == teleport.JSON {
			return trace.Wrap(utils.WriteJSON(cf.Stdout(), payload))
		}
		return trace.Wrap(utils.WriteYAML(cf.Stdout(), payload))
	case teleport.Text, "":
	default:
		return trace.BadParameter("unsupported format %q. Use text, json, or yaml", cf.Format)
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
		if len(logins) > 0 {
			// Apps have AWS role ARNs in the same flat 'Logins' field
			heading := "Logins"
			if id.Kind == types.KindApp {
				heading = "Role ARNs"
			}
			fmt.Fprintf(w, "\n%s:\n", heading)
			for _, p := range logins {
				fmt.Fprintf(w, "  %s\n", p)
			}
			fmt.Fprint(w, "\nThis cluster does not report which of these are already granted.\n")
		} else {
			fmt.Fprint(w, "\nNo principal information is available for this resource. Request it without constraints.\n")
		}
		fmt.Fprintf(w, "\nhint: tsh request create --resource %s --reason \"...\"\n", quotedID)
		return nil
	}

	principalTypes := slices.Sorted(maps.Keys(splits))

	// Role ARNs are too long for a fixed column width.
	width := 0
	hasRequestable := false
	for _, s := range splits {
		hasRequestable = hasRequestable || len(s.requestable) > 0
		for _, p := range slices.Concat(s.granted, s.requestable) {
			width = max(width, len(p))
		}
	}

	for _, principalType := range principalTypes {
		s := splits[principalType]
		if len(s.granted) == 0 && len(s.requestable) == 0 {
			continue
		}
		heading := principalType
		switch principalType {
		case types.PrincipalTypeLogins:
			heading = "Logins"
		case types.PrincipalTypeRoleARNs:
			heading = "Role ARNs"
		}
		fmt.Fprintf(w, "\n%s:\n", heading)
		for _, p := range s.granted {
			fmt.Fprintf(w, "  %-*s  granted\n", width, p)
		}
		for _, p := range s.requestable {
			fmt.Fprintf(w, "  %-*s  requestable\n", width, p)
		}
	}

	if !hasRequestable {
		fmt.Fprint(w, "\nNo additional principals are requestable with the current identity.\n")
		return nil
	}
	resource := idStr + hintConstraintSuffix(id.Kind, principalTypes, splits)
	fmt.Fprintf(w, "\nhint: tsh request create --resource '%s' --reason \"...\"\n", strings.ReplaceAll(resource, "'", `'\''`))
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

// Load the timezone from the TZ env variable or use the default timezone if TZ is not set. If the
// timezone cannot be loaded, return the time in UTC.
// NOTE: the TZ env variable MUST be in IANA format (eg. "Europe/London" or "America/Los_Angeles").
// If the TZ env variable is set to a non-IANA format (eg. "PST" or "EST"), the timezone will not be
// loaded and the time will be returned in UTC.
func timeZoneFromEnvOr(ts time.Time, defaultTZ string) time.Time {
	if ts.IsZero() {
		return ts
	}

	tz := os.Getenv("TZ")
	if tz == "" {
		tz = defaultTZ
	}

	timeZone, err := time.LoadLocation(tz)
	if err != nil {
		return ts.UTC()
	}

	return ts.In(timeZone)
}

func timeZoneFromEnvOrLocal(ts time.Time) time.Time {
	return timeZoneFromEnvOr(ts, "Local")
}
