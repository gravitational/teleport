/*
Copyright 2019 Gravitational, Inc.

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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	libauth "github.com/gravitational/teleport/lib/auth"
	auth "github.com/gravitational/teleport/lib/auth/client"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// AccessRequestCommand implements `tctl users` set of commands
// It implements CLICommand interface
type AccessRequestCommand struct {
	config *service.Config
	reqIDs string

	user        string
	roles       string
	delegator   string
	reason      string
	annotations string
	// format is the output format, e.g. text or json
	format string

	dryRun bool

	approve, deny bool

	requestList    *kingpin.CmdClause
	requestGet     *kingpin.CmdClause
	requestApprove *kingpin.CmdClause
	requestDeny    *kingpin.CmdClause
	requestCreate  *kingpin.CmdClause
	requestDelete  *kingpin.CmdClause
	requestCaps    *kingpin.CmdClause
	requestReview  *kingpin.CmdClause
}

// Initialize allows AccessRequestCommand to plug itself into the CLI parser
func (c *AccessRequestCommand) Initialize(app *kingpin.Application, config *service.Config) {
	c.config = config
	requests := app.Command("requests", "Manage access requests").Alias("request")

	c.requestList = requests.Command("ls", "Show active access requests")
	c.requestList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)

	c.requestGet = requests.Command("get", "Show access request by ID")
	c.requestGet.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)
	c.requestGet.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)

	c.requestApprove = requests.Command("approve", "Approve pending access request")
	c.requestApprove.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)
	c.requestApprove.Flag("delegator", "Optional delegating identity").StringVar(&c.delegator)
	c.requestApprove.Flag("reason", "Optional reason message").StringVar(&c.reason)
	c.requestApprove.Flag("annotations", "Resolution attributes <key>=<val>[,...]").StringVar(&c.annotations)
	c.requestApprove.Flag("roles", "Override requested roles <role>[,...]").StringVar(&c.roles)

	c.requestDeny = requests.Command("deny", "Deny pending access request")
	c.requestDeny.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)
	c.requestDeny.Flag("delegator", "Optional delegating identity").StringVar(&c.delegator)
	c.requestDeny.Flag("reason", "Optional reason message").StringVar(&c.reason)
	c.requestDeny.Flag("annotations", "Resolution annotations <key>=<val>[,...]").StringVar(&c.annotations)

	c.requestCreate = requests.Command("create", "Create pending access request")
	c.requestCreate.Arg("username", "Name of target user").Required().StringVar(&c.user)
	c.requestCreate.Flag("roles", "Roles to be requested").Default("*").StringVar(&c.roles)
	c.requestCreate.Flag("reason", "Optional reason message").StringVar(&c.reason)
	c.requestCreate.Flag("dry-run", "Don't actually generate the access request").BoolVar(&c.dryRun)

	c.requestDelete = requests.Command("rm", "Delete an access request")
	c.requestDelete.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)

	c.requestCaps = requests.Command("capabilities", "Check a user's access capabilities").Alias("caps").Hidden()
	c.requestCaps.Arg("username", "Name of target user").Required().StringVar(&c.user)
	c.requestCaps.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)
	c.requestReview = requests.Command("review", "Review an access request")
	c.requestReview.Arg("request-id", "ID of target request").Required().StringVar(&c.reqIDs)
	c.requestReview.Flag("author", "Username of reviewer").Required().StringVar(&c.user)
	c.requestReview.Flag("approve", "Review proposes approval").BoolVar(&c.approve)
	c.requestReview.Flag("deny", "Review proposes denial").BoolVar(&c.deny)
}

// TryRun takes the CLI command as an argument (like "access-request list") and executes it.
func (c *AccessRequestCommand) TryRun(cmd string, client auth.ClientI) (match bool, err error) {
	switch cmd {
	case c.requestList.FullCommand():
		err = c.List(client)
	case c.requestGet.FullCommand():
		err = c.Get(client)
	case c.requestApprove.FullCommand():
		err = c.Approve(client)
	case c.requestDeny.FullCommand():
		err = c.Deny(client)
	case c.requestCreate.FullCommand():
		err = c.Create(client)
	case c.requestDelete.FullCommand():
		err = c.Delete(client)
	case c.requestCaps.FullCommand():
		err = c.Caps(client)
	case c.requestReview.FullCommand():
		err = c.Review(client)
	default:
		return false, nil
	}
	return true, trace.Wrap(err)
}

func (c *AccessRequestCommand) List(client auth.ClientI) error {
	reqs, err := client.GetAccessRequests(context.TODO(), services.AccessRequestFilter{})
	if err != nil {
		return trace.Wrap(err)
	}

	now := time.Now()
	activeReqs := []services.AccessRequest{}
	for _, req := range reqs {
		if now.Before(req.GetAccessExpiry()) {
			activeReqs = append(activeReqs, req)
		}
	}
	sort.Slice(activeReqs, func(i, j int) bool {
		return activeReqs[i].GetCreationTime().After(activeReqs[j].GetCreationTime())
	})

	if err := printRequestsOverview(activeReqs, c.format); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *AccessRequestCommand) Get(client auth.ClientI) error {
	ctx := context.TODO()
	reqs := []services.AccessRequest{}
	for _, reqID := range strings.Split(c.reqIDs, ",") {
		req, err := client.GetAccessRequests(ctx, services.AccessRequestFilter{
			ID: reqID,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if len(req) != 1 {
			return trace.BadParameter("request with ID %q not found", reqID)
		}
		reqs = append(reqs, req...)
	}
	if err := printRequestsDetailed(reqs, c.format); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *AccessRequestCommand) splitAnnotations() (map[string][]string, error) {
	annotations := make(map[string][]string)
	for _, s := range strings.Split(c.annotations, ",") {
		if s == "" {
			continue
		}
		idx := strings.Index(s, "=")
		if idx < 1 {
			return nil, trace.BadParameter("invalid key-value pair: %q", s)
		}
		key, val := strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+1:])
		if key == "" {
			return nil, trace.BadParameter("empty attr key")
		}
		if val == "" {
			return nil, trace.BadParameter("empty sttr val")
		}
		vals := annotations[key]
		vals = append(vals, val)
		annotations[key] = vals
	}
	return annotations, nil
}

func (c *AccessRequestCommand) splitRoles() []string {
	var roles []string
	for _, s := range strings.Split(c.roles, ",") {
		if s == "" {
			continue
		}
		roles = append(roles, s)
	}
	return roles
}

func (c *AccessRequestCommand) Approve(client auth.ClientI) error {
	ctx := context.TODO()
	if c.delegator != "" {
		ctx = apiclient.WithDelegator(ctx, c.delegator)
	}
	annotations, err := c.splitAnnotations()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, reqID := range strings.Split(c.reqIDs, ",") {
		if err := client.SetAccessRequestState(ctx, services.AccessRequestUpdate{
			RequestID:   reqID,
			State:       services.RequestState_APPROVED,
			Reason:      c.reason,
			Annotations: annotations,
			Roles:       c.splitRoles(),
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *AccessRequestCommand) Deny(client auth.ClientI) error {
	ctx := context.TODO()
	if c.delegator != "" {
		ctx = apiclient.WithDelegator(ctx, c.delegator)
	}
	annotations, err := c.splitAnnotations()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, reqID := range strings.Split(c.reqIDs, ",") {
		if err := client.SetAccessRequestState(ctx, services.AccessRequestUpdate{
			RequestID:   reqID,
			State:       services.RequestState_DENIED,
			Reason:      c.reason,
			Annotations: annotations,
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *AccessRequestCommand) Create(client auth.ClientI) error {
	roles := c.splitRoles()
	if len(roles) == 0 {
		return trace.BadParameter("need at least one role")
	}
	req, err := libauth.NewAccessRequest(c.user, roles[0], roles[1:]...)
	if err != nil {
		return trace.Wrap(err)
	}
	req.SetRequestReason(c.reason)

	if c.dryRun {
		err = libauth.ValidateAccessRequestForUser(client, req, libauth.ExpandVars(true))
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(printJSON(req, "request"))
	}
	if err := client.CreateAccessRequest(context.TODO(), req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s\n", req.GetName())
	return nil
}

func (c *AccessRequestCommand) Delete(client auth.ClientI) error {
	for _, reqID := range strings.Split(c.reqIDs, ",") {
		if err := client.DeleteAccessRequest(context.TODO(), reqID); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *AccessRequestCommand) Caps(client auth.ClientI) error {
	caps, err := client.GetAccessCapabilities(context.TODO(), services.AccessCapabilitiesRequest{
		User:               c.user,
		RequestableRoles:   true,
		SuggestedReviewers: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	switch c.format {
	case teleport.Text:
		// represent capabilities as a simple key-value table
		table := asciitable.MakeTable([]string{"Name", "Value"})

		// populate requestable roles
		rr := "None"
		if len(caps.RequestableRoles) > 0 {
			rr = strings.Join(caps.RequestableRoles, ",")
		}
		table.AddRow([]string{"Requestable Roles:", rr})

		sr := "None"
		if len(caps.SuggestedReviewers) > 0 {
			sr = strings.Join(caps.SuggestedReviewers, ",")
		}
		table.AddRow([]string{"Suggested Reviewers:", sr})

		_, err := table.AsBuffer().WriteTo(os.Stdout)
		return trace.Wrap(err)
	case teleport.JSON:
		return printJSON(caps, "capabilities")
	default:
		return trace.BadParameter("unknown format %q, must be one of [%q, %q]", c.format, teleport.Text, teleport.JSON)
	}
}

func (c *AccessRequestCommand) Review(client auth.ClientI) error {
	if c.approve == c.deny {
		return trace.BadParameter("must supply exactly one of '--approve' or '--deny'")
	}

	var state types.RequestState
	switch {
	case c.approve:
		state = types.RequestState_APPROVED
	case c.deny:
		state = types.RequestState_DENIED
	}

	ctx := context.TODO()

	req, err := client.SubmitAccessReview(ctx, types.AccessReviewSubmission{
		RequestID: strings.Split(c.reqIDs, ",")[0],
		Review: types.AccessReview{
			Author:        c.user,
			ProposedState: state,
		},
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

// printRequestsOverview prints an overview of given access requests.
func printRequestsOverview(reqs []services.AccessRequest, format string) error {
	switch format {
	case teleport.Text:
		table := asciitable.MakeTable([]string{"Token", "Requestor", "Metadata", "Created At (UTC)", "Status"})
		table.AddColumn(asciitable.Column{
			Title:         "Request Reason",
			MaxCellLength: 75,
			FootnoteLabel: "[*]",
		})
		table.AddColumn(asciitable.Column{
			Title:         "Resolve Reason",
			MaxCellLength: 75,
			FootnoteLabel: "[*]",
		})
		table.AddFootnote(
			"[*]",
			"Full reason was truncated, use the `tctl requests get` subcommand to view the full reason.",
		)
		for _, req := range reqs {
			table.AddRow([]string{
				req.GetName(),
				req.GetUser(),
				fmt.Sprintf("roles=%s", strings.Join(req.GetRoles(), ",")),
				req.GetCreationTime().Format(time.RFC822),
				req.GetState().String(),
				req.GetRequestReason(),
				req.GetResolveReason(),
			})
		}
		_, err := table.AsBuffer().WriteTo(os.Stdout)
		return trace.Wrap(err)
	case teleport.JSON:
		return printJSON(reqs, "requests")
	default:
		return trace.BadParameter("unknown format %q, must be one of [%q, %q]", format, teleport.Text, teleport.JSON)
	}
}

// printRequestsDetailed prints a detailed view of given access requests.
func printRequestsDetailed(reqs []services.AccessRequest, format string) error {
	switch format {
	case teleport.Text:
		for _, req := range reqs {
			table := asciitable.MakeHeadlessTable(2)
			table.AddRow([]string{"Token: ", req.GetName()})
			table.AddRow([]string{"Requestor: ", req.GetUser()})
			table.AddRow([]string{"Metadata: ", fmt.Sprintf("roles=%s", strings.Join(req.GetRoles(), ","))})
			table.AddRow([]string{"Created At (UTC): ", req.GetCreationTime().Format(time.RFC822)})
			table.AddRow([]string{"Status: ", req.GetState().String()})
			table.AddRow([]string{"Request Reason: ", req.GetRequestReason()})
			table.AddRow([]string{"Resolve Reason: ", req.GetResolveReason()})

			_, err := table.AsBuffer().WriteTo(os.Stdout)
			if err != nil {
				return trace.Wrap(err)
			}
			fmt.Println()
		}
		return nil
	case teleport.JSON:
		return printJSON(reqs, "requests")
	default:
		return trace.BadParameter("unknown format %q, must be one of [%q, %q]", format, teleport.Text, teleport.JSON)
	}
}

func printJSON(in interface{}, desc string) error {
	out, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return trace.Wrap(err, fmt.Sprintf("failed to marshal %v", desc))
	}
	fmt.Printf("%s\n", out)
	return nil
}
