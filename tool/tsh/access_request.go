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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

func onRequestList(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
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
	switch cf.Format {
	case teleport.Text:
		if err := showRequestTable(reqs); err != nil {
			return trace.Wrap(err)
		}
	case teleport.JSON:
		ser, err := json.MarshalIndent(reqs, "", "  ")
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("%s\n", ser)
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}
	return nil
}

func onRequestShow(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
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

	reason := "[none]"
	if r := req.GetRequestReason(); r != "" {
		reason = fmt.Sprintf("%q", r)
	}

	reviewers := "[none]"
	if r := req.GetSuggestedReviewers(); len(r) > 0 {
		reviewers = strings.Join(r, ", ")
	}

	table := asciitable.MakeHeadlessTable(2)
	table.AddRow([]string{"Request ID:", req.GetName()})
	table.AddRow([]string{"Username:", req.GetUser()})
	table.AddRow([]string{"Roles:", strings.Join(req.GetRoles(), ", ")})
	table.AddRow([]string{"Reason:", reason})
	table.AddRow([]string{"Reviewers:", reviewers + " (suggested)"})
	table.AddRow([]string{"Status:", req.GetState().String()})

	_, err = table.AsBuffer().WriteTo(os.Stdout)
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
		fmt.Println("------------------------------------------------")
		fmt.Printf("%s:\n", title)

		for _, rev := range revs {
			fmt.Println("  ----------------------------------------------")

			revReason := "[none]"
			if rev.Reason != "" {
				revReason = fmt.Sprintf("%q", rev.Reason)
			}

			subTable := asciitable.MakeHeadlessTable(2)
			subTable.AddRow([]string{"  Reviewer:", rev.Author})
			subTable.AddRow([]string{"  Reason:", revReason})
			_, err = subTable.AsBuffer().WriteTo(os.Stdout)
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
	return nil
}

func onRequestCreate(cf *CLIConf) error {
	tc, err := makeClient(cf, true)
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
	tc, err := makeClient(cf, false)
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
		fmt.Fprintf(os.Stderr, "Successfully submitted review.  Request state: %s\n", req.GetState())
	} else {
		fmt.Fprintf(os.Stderr, "Warning: ineffectual review. Request state: %s\n", req.GetState())
	}
	return nil
}

func showRequestTable(reqs []types.AccessRequest) error {
	sort.Slice(reqs, func(i, j int) bool {
		return reqs[i].GetCreationTime().After(reqs[j].GetCreationTime())
	})

	table := asciitable.MakeTable([]string{"ID", "User", "Roles", "Created (UTC)", "Status"})
	now := time.Now()
	for _, req := range reqs {
		if now.After(req.GetAccessExpiry()) {
			continue
		}
		table.AddRow([]string{
			req.GetName(),
			req.GetUser(),
			strings.Join(req.GetRoles(), ","),
			req.GetCreationTime().UTC().Format(time.RFC822),
			req.GetState().String(),
		})
	}
	_, err := table.AsBuffer().WriteTo(os.Stdout)

	fmt.Fprintf(os.Stderr, "\nhint: use 'tsh request show <request-id>' for additional details\n")
	return trace.Wrap(err)
}
