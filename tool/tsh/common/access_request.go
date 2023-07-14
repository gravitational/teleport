/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	kubeproto "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

var requestLoginHint = "use 'tsh login --request-id=<request-id>' to login with an approved request"

func onRequestList(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if cf.Username == "" {
		cf.Username = tc.Username
	}

	var reqs []types.AccessRequest

	err = tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
		reqs, err = clt.GetAccessRequests(cf.Context, types.AccessRequestFilter{})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if cf.ReviewableRequests {
		filtered := reqs[:0]
	Reviewable:
		for _, req := range reqs {
			if req.GetUser() == cf.Username {
				continue Reviewable
			}
			for _, rev := range req.GetReviews() {
				if rev.Author == cf.Username {
					continue Reviewable
				}
			}
			filtered = append(filtered, req)
		}
		reqs = filtered
	}
	if cf.SuggestedRequests {
		filtered := reqs[:0]
	Suggested:
		for _, req := range reqs {
			if req.GetUser() == cf.Username {
				continue Suggested
			}
			for _, rev := range req.GetReviews() {
				if rev.Author == cf.Username {
					continue Suggested
				}
			}
			for _, reviewer := range req.GetSuggestedReviewers() {
				if reviewer == cf.Username {
					filtered = append(filtered, req)
					continue Suggested
				}
			}
		}
		reqs = filtered
	}
	if cf.MyRequests {
		filtered := reqs[:0]
		for _, req := range reqs {
			if req.GetUser() == cf.Username {
				filtered = append(filtered, req)
			}
		}
		reqs = filtered
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
	err = tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
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

	resourcesStr := ""
	if resources := req.GetRequestedResourceIDs(); len(resources) > 0 {
		var err error
		if resourcesStr, err = types.ResourceIDsToString(resources); err != nil {
			return trace.Wrap(err)
		}
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
	table.AddRow([]string{"Status:", req.GetState().String()})

	_, err := table.AsBuffer().WriteTo(cf.Stdout())
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
		fmt.Fprint(cf.Stdout(), "------------------------------------------------")
		fmt.Fprintf(cf.Stdout(), "%s:\n", title)

		for _, rev := range revs {
			fmt.Fprint(cf.Stdout(), "  ----------------------------------------------")

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

	var state types.RequestState
	switch {
	case cf.Approve:
		state = types.RequestState_APPROVED
	case cf.Deny:
		state = types.RequestState_DENIED
	}

	var req types.AccessRequest
	err = tc.WithRootClusterClient(cf.Context, func(clt auth.ClientI) error {
		req, err = clt.SubmitAccessReview(cf.Context, types.AccessReviewSubmission{
			RequestID: cf.RequestID,
			Review: types.AccessReview{
				Author:        cf.Username,
				ProposedState: state,
				Reason:        cf.ReviewReason,
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

	table := asciitable.MakeTable([]string{"ID", "User", "Roles"})
	table.AddColumn(asciitable.Column{
		Title:         "Resources",
		MaxCellLength: 20,
		FootnoteLabel: "[+]",
	})
	table.AddFootnote("[+]",
		"Requested resources truncated, use `tsh request show <request-id>` to view the full list")
	table.AddColumn(asciitable.Column{Title: "Created At (UTC)"})
	table.AddColumn(asciitable.Column{Title: "Request TTL"})
	table.AddColumn(asciitable.Column{Title: "Session TTL"})
	table.AddColumn(asciitable.Column{Title: "Status"})
	now := time.Now()
	for _, req := range reqs {
		if now.After(req.GetAccessExpiry()) {
			continue
		}
		resourceIDsString, err := types.ResourceIDsToString(req.GetRequestedResourceIDs())
		if err != nil {
			return trace.Wrap(err)
		}
		table.AddRow([]string{
			req.GetName(),
			req.GetUser(),
			strings.Join(req.GetRoles(), ","),
			resourceIDsString,
			req.GetCreationTime().UTC().Format(time.RFC822),
			time.Until(req.Expiry()).Round(time.Minute).String(),
			time.Until(req.GetAccessExpiry()).Round(time.Minute).String(),
			req.GetState().String(),
		})
	}
	_, err := table.AsBuffer().WriteTo(cf.Stdout())

	fmt.Fprintf(cf.Stdout(), "\nhint: use 'tsh request show <request-id>' for additional details\n")
	fmt.Fprintf(cf.Stdout(), "      %v\n", requestLoginHint)
	return trace.Wrap(err)
}

func onRequestSearch(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// If KubeCluster not provided try to read it from kubeconfig.
	if cf.KubernetesCluster == "" {
		cf.KubernetesCluster = selectedKubeCluster(tc.SiteName)
	}
	if slices.Contains(types.KubernetesResourcesKinds, cf.ResourceKind) && cf.KubernetesCluster == "" {
		return trace.BadParameter("when searching for Pods, --kube-cluster cannot be empty")
	}
	// if --all-namespaces flag was provided we search in every namespace.
	// This means sending an empty namespace to the ListResources API.
	if cf.kubeAllNamespaces {
		cf.kubeNamespace = ""
	}

	var resources types.ResourcesWithLabels
	var tableColumns []string
	switch {
	case slices.Contains(types.KubernetesResourcesKinds, cf.ResourceKind):
		proxyGRPCClient, err := tc.NewKubernetesServiceClient(cf.Context, tc.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		req := kubeproto.ListKubernetesResourcesRequest{
			ResourceType:        cf.ResourceKind,
			Labels:              tc.Labels,
			PredicateExpression: cf.PredicateExpression,
			SearchKeywords:      tc.SearchKeywords,
			UseSearchAsRoles:    true,
			KubernetesCluster:   cf.KubernetesCluster,
			KubernetesNamespace: cf.kubeNamespace,
			TeleportCluster:     tc.SiteName,
		}

		resources, err = client.GetKubernetesResourcesWithFilters(cf.Context, proxyGRPCClient, &req)
		if err != nil {
			return trace.Wrap(err)
		}

		tableColumns = []string{"Name", "Namespace", "Labels", "Resource ID"}
	default:
		// For all other resources, we need to connect to the auth server.
		proxyClient, err := tc.ConnectToProxy(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient := proxyClient.CurrentCluster()

		req := proto.ListResourcesRequest{
			ResourceType:        services.MapResourceKindToListResourcesType(cf.ResourceKind),
			Labels:              tc.Labels,
			PredicateExpression: cf.PredicateExpression,
			SearchKeywords:      tc.SearchKeywords,
			UseSearchAsRoles:    true,
		}

		results, err := client.GetResourcesWithFilters(cf.Context, authClient, req)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, result := range results {
			leafResources, err := services.MapListResourcesResultToLeafResource(result, cf.ResourceKind)
			if err != nil {
				return trace.Wrap(err)
			}
			resources = append(resources, leafResources...)
		}
		tableColumns = []string{"Name", "Hostname", "Labels", "Resource ID"}
	}

	var rows [][]string
	var resourceIDs []string
	deduplicateResourceIDs := map[string]struct{}{}
	for _, resource := range resources {
		var row []string
		switch r := resource.(type) {
		case *types.KubernetesResourceV1:
			resourceID := types.ResourceIDToString(types.ResourceID{
				ClusterName:     tc.SiteName,
				Kind:            resource.GetKind(),
				Name:            cf.KubernetesCluster,
				SubResourceName: path.Join(r.Spec.Namespace, resource.GetName()),
			})
			if ignoreDuplicateResourceId(deduplicateResourceIDs, resourceID) {
				continue
			}
			resourceIDs = append(resourceIDs, resourceID)

			row = []string{
				resource.GetName(),
				r.Spec.Namespace,
				sortedLabels(resource.GetAllLabels()),
				resourceID,
			}

		default:
			resourceID := types.ResourceIDToString(types.ResourceID{
				ClusterName: tc.SiteName,
				Kind:        resource.GetKind(),
				Name:        resource.GetName(),
			})
			if ignoreDuplicateResourceId(deduplicateResourceIDs, resourceID) {
				continue
			}

			resourceIDs = append(resourceIDs, resourceID)
			hostName := ""
			if r, ok := resource.(interface{ GetHostname() string }); ok {
				hostName = r.GetHostname()
			}
			row = []string{
				resource.GetName(),
				hostName,
				sortedLabels(resource.GetAllLabels()),
				resourceID,
			}
		}
		rows = append(rows, row)
	}
	table := asciitable.MakeTableWithTruncatedColumn(tableColumns, rows, "Labels")
	if _, err := table.AsBuffer().WriteTo(cf.Stdout()); err != nil {
		return trace.Wrap(err)
	}

	if len(resourceIDs) > 0 {
		resourcesStr := strings.Join(resourceIDs, " --resource ")
		fmt.Fprintf(cf.Stdout(), `
To request access to these resources, run
> tsh request create --resource %s \
    --reason <request reason>

`, resourcesStr)
	}

	return nil
}

// ignoreDuplicateResourceId returns true if the resource ID is a duplicate
// and should be ignored. Otherwise, it returns false and adds the resource ID
// to the deduplicateResourceIDs map.
func ignoreDuplicateResourceId(deduplicateResourceIDs map[string]struct{}, resourceID string) bool {
	// Ignore duplicate resource IDs.
	if _, ok := deduplicateResourceIDs[resourceID]; ok {
		return true
	}
	deduplicateResourceIDs[resourceID] = struct{}{}
	return false
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
