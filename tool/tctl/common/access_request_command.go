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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// AccessRequestCommand implements `tctl users` set of commands
// It implements CLICommand interface
type AccessRequestCommand struct {
	config *servicecfg.Config
	reqIDs string

	user                 string
	roles                string
	requestedResourceIDs []string
	delegator            string
	reason               string
	annotations          string
	// format is the output format, e.g. text or json
	format string

	dryRun bool
	force  bool

	approve, deny bool
	// assumeStartTimeRaw format is RFC3339
	assumeStartTimeRaw string

	sortOrder, sortIndex string

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
func (c *AccessRequestCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.config = config
	requests := app.Command("requests", "Manage access requests.").Alias("request")

	c.requestList = requests.Command("ls", "Show active access requests.")
	c.requestList.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)
	c.requestList.Flag("sort-index", "Request sort index, 'created' or 'state'").Default("created").StringVar(&c.sortIndex)
	c.requestList.Flag("sort-order", "Request sort order, 'ascending' or 'descending'").Default("descending").StringVar(&c.sortOrder)

	c.requestGet = requests.Command("get", "Show access request by ID.")
	c.requestGet.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)
	c.requestGet.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)

	c.requestApprove = requests.Command("approve", "Approve pending access request.")
	c.requestApprove.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)
	c.requestApprove.Flag("delegator", "Optional delegating identity").StringVar(&c.delegator)
	c.requestApprove.Flag("reason", "Optional reason message").StringVar(&c.reason)
	c.requestApprove.Flag("annotations", "Resolution attributes <key>=<val>[,...]").StringVar(&c.annotations)
	c.requestApprove.Flag("roles", "Override requested roles <role>[,...]").StringVar(&c.roles)
	c.requestApprove.Flag("assume-start-time", "Sets time roles can be assumed by requestor (RFC3339 e.g 2023-12-12T23:20:50.52Z)").StringVar(&c.assumeStartTimeRaw)

	c.requestDeny = requests.Command("deny", "Deny pending access request.")
	c.requestDeny.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)
	c.requestDeny.Flag("delegator", "Optional delegating identity").StringVar(&c.delegator)
	c.requestDeny.Flag("reason", "Optional reason message").StringVar(&c.reason)
	c.requestDeny.Flag("annotations", "Resolution annotations <key>=<val>[,...]").StringVar(&c.annotations)

	c.requestCreate = requests.Command("create", "Create pending access request.")
	c.requestCreate.Arg("username", "Name of target user").Required().StringVar(&c.user)
	c.requestCreate.Flag("roles", "Roles to be requested").StringVar(&c.roles)
	c.requestCreate.Flag("resource", "Resource ID to be requested").StringsVar(&c.requestedResourceIDs)
	c.requestCreate.Flag("reason", "Optional reason message").StringVar(&c.reason)
	c.requestCreate.Flag("dry-run", "Don't actually generate the access request").BoolVar(&c.dryRun)

	c.requestDelete = requests.Command("rm", "Delete an access request.")
	c.requestDelete.Arg("request-id", "ID of target request(s)").Required().StringVar(&c.reqIDs)
	c.requestDelete.Flag("force", "Force the deletion of an active access request").Short('f').BoolVar(&c.force)

	c.requestCaps = requests.Command("capabilities", "Check a user's access capabilities.").Alias("caps").Hidden()
	c.requestCaps.Arg("username", "Name of target user").Required().StringVar(&c.user)
	c.requestCaps.Flag("format", "Output format, 'text' or 'json'").Hidden().Default(teleport.Text).StringVar(&c.format)
	c.requestReview = requests.Command("review", "Review an access request.")
	c.requestReview.Arg("request-id", "ID of target request").Required().StringVar(&c.reqIDs)
	c.requestReview.Flag("author", "Username of reviewer").Required().StringVar(&c.user)
	c.requestReview.Flag("approve", "Review proposes approval").BoolVar(&c.approve)
	c.requestReview.Flag("deny", "Review proposes denial").BoolVar(&c.deny)
}

// TryRun takes the CLI command as an argument (like "access-request list") and executes it.
func (c *AccessRequestCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(ctx context.Context, client *authclient.Client) error
	switch cmd {
	case c.requestList.FullCommand():
		commandFunc = c.List
	case c.requestGet.FullCommand():
		commandFunc = c.Get
	case c.requestApprove.FullCommand():
		commandFunc = c.Approve
	case c.requestDeny.FullCommand():
		commandFunc = c.Deny
	case c.requestCreate.FullCommand():
		commandFunc = c.Create
	case c.requestDelete.FullCommand():
		commandFunc = c.Delete
	case c.requestCaps.FullCommand():
		commandFunc = c.Caps
	case c.requestReview.FullCommand():
		commandFunc = c.Review
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	err = commandFunc(ctx, client)
	closeFn(ctx)

	return true, trace.Wrap(err)
}

func (c *AccessRequestCommand) List(ctx context.Context, client *authclient.Client) error {
	var index proto.AccessRequestSort
	switch c.sortIndex {
	case "created":
		index = proto.AccessRequestSort_CREATED
	case "state":
		index = proto.AccessRequestSort_STATE
	default:
		return trace.BadParameter("unsupported sort index %q (expected one of 'created' or 'state')", c.sortIndex)
	}

	var descending bool
	switch c.sortOrder {
	case "ascending":
		descending = false
	case "descending":
		descending = true
	default:
		return trace.BadParameter("unsupported sort order %q (expected one of 'ascending' or 'descending')", c.sortOrder)
	}

	reqs, err := client.ListAllAccessRequests(ctx, &proto.ListAccessRequestsRequest{
		Sort:       index,
		Descending: descending,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	now := time.Now()
	activeReqs := []types.AccessRequest{}
	for _, req := range reqs {
		if now.Before(req.GetAccessExpiry()) {
			activeReqs = append(activeReqs, req)
		}
	}

	if err := printRequestsOverview(activeReqs, c.format); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *AccessRequestCommand) Get(ctx context.Context, client *authclient.Client) error {
	reqs := []types.AccessRequest{}
	for reqID := range strings.SplitSeq(c.reqIDs, ",") {
		req, err := client.GetAccessRequests(ctx, types.AccessRequestFilter{
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
	for s := range strings.SplitSeq(c.annotations, ",") {
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
	for s := range strings.SplitSeq(c.roles, ",") {
		if s == "" {
			continue
		}
		roles = append(roles, s)
	}
	return roles
}

func (c *AccessRequestCommand) Approve(ctx context.Context, client *authclient.Client) error {
	if c.delegator != "" {
		ctx = authz.WithDelegator(ctx, c.delegator)
	}
	annotations, err := c.splitAnnotations()
	if err != nil {
		return trace.Wrap(err)
	}
	var assumeStartTime *time.Time
	if c.assumeStartTimeRaw != "" {
		parsedAssumeStartTime, err := time.Parse(time.RFC3339, c.assumeStartTimeRaw)
		if err != nil {
			return trace.BadParameter("parsing assume-start-time (required format RFC3339 e.g 2023-12-12T23:20:50.52Z): %v", err)
		}
		assumeStartTime = &parsedAssumeStartTime
	}
	for reqID := range strings.SplitSeq(c.reqIDs, ",") {
		if err := client.SetAccessRequestState(ctx, types.AccessRequestUpdate{
			RequestID:       reqID,
			State:           types.RequestState_APPROVED,
			Reason:          c.reason,
			Annotations:     annotations,
			Roles:           c.splitRoles(),
			AssumeStartTime: assumeStartTime,
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *AccessRequestCommand) Deny(ctx context.Context, client *authclient.Client) error {
	if c.delegator != "" {
		ctx = authz.WithDelegator(ctx, c.delegator)
	}
	annotations, err := c.splitAnnotations()
	if err != nil {
		return trace.Wrap(err)
	}
	for reqID := range strings.SplitSeq(c.reqIDs, ",") {
		if err := client.SetAccessRequestState(ctx, types.AccessRequestUpdate{
			RequestID:   reqID,
			State:       types.RequestState_DENIED,
			Reason:      c.reason,
			Annotations: annotations,
		}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *AccessRequestCommand) Create(ctx context.Context, client *authclient.Client) error {
	if len(c.roles) == 0 && len(c.requestedResourceIDs) == 0 {
		c.roles = "*"
	}
	requestedResourceIDs, err := types.ResourceIDsFromStrings(c.requestedResourceIDs)
	if err != nil {
		return trace.Wrap(err)
	}
	req, err := services.NewAccessRequestWithResources(c.user, c.splitRoles(), requestedResourceIDs)
	if err != nil {
		return trace.Wrap(err)
	}
	req.SetRequestReason(c.reason)

	if c.dryRun {
		users := &struct {
			*authclient.Client
			services.UserLoginStatesGetter
		}{
			Client:                client,
			UserLoginStatesGetter: client.UserLoginStateClient(),
		}
		err = services.ValidateAccessRequestForUser(ctx, clockwork.NewRealClock(), users, req, tlsca.Identity{}, services.WithExpandVars(true))
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(printJSON(req, "request"))
	}
	req, err = client.CreateAccessRequestV2(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%s\n", req.GetName())
	return nil
}

func (c *AccessRequestCommand) Delete(ctx context.Context, client *authclient.Client) error {
	var approvedTokens []string
	for reqID := range strings.SplitSeq(c.reqIDs, ",") {
		// Fetch the requests first to see if they were approved to provide the
		// proper messaging.
		reqs, err := client.GetAccessRequests(ctx, types.AccessRequestFilter{
			ID: reqID,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if len(reqs) != 1 {
			return trace.BadParameter("request with ID %q not found", reqID)
		}
		if reqs[0].GetState().String() == "APPROVED" {
			approvedTokens = append(approvedTokens, reqID)
		}
	}

	if len(approvedTokens) == 0 || c.force {
		for reqID := range strings.SplitSeq(c.reqIDs, ",") {
			if err := client.DeleteAccessRequest(ctx, reqID); err != nil {
				return trace.Wrap(err)
			}
		}
		fmt.Println("Access request deleted successfully.")
	}

	if !c.force && len(approvedTokens) > 0 {
		fmt.Println("\nThis access request has already been approved, deleting the request now will NOT remove")
		fmt.Println("the user's access to these roles. If you would like to lock the user's access to the")
		fmt.Printf("requested roles instead, you can run:\n\n")
		for _, reqID := range approvedTokens {
			fmt.Printf("> tctl lock --access-request %s\n", reqID)
		}
		fmt.Printf("\nTo disregard this warning and delete the request anyway, re-run this command with --force.\n\n")
	}
	return nil
}

func (c *AccessRequestCommand) Caps(ctx context.Context, client *authclient.Client) error {
	caps, err := client.GetAccessCapabilities(ctx, types.AccessCapabilitiesRequest{
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

func (c *AccessRequestCommand) Review(ctx context.Context, client *authclient.Client) error {
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
func printRequestsOverview(reqs []types.AccessRequest, format string) error {
	switch format {
	case teleport.Text:
		table := asciitable.MakeTable([]string{"Token", "Requestor", "Metadata"})
		table.AddColumn(asciitable.Column{
			Title:         "Resources",
			MaxCellLength: 20,
			FootnoteLabel: "[+]",
		})
		table.AddFootnote(
			"[+]",
			"Requested resources truncated, use the `tctl requests get` subcommand to view the full list")
		table.AddColumn(asciitable.Column{Title: "Created At (UTC)"})
		table.AddColumn(asciitable.Column{Title: "Request TTL"})
		table.AddColumn(asciitable.Column{Title: "Session TTL"})
		table.AddColumn(asciitable.Column{Title: "Status"})
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
			resourceIDsString, err := types.ResourceIDsToString(req.GetRequestedResourceIDs())
			if err != nil {
				return trace.Wrap(err)
			}
			table.AddRow([]string{
				req.GetName(),
				req.GetUser(),
				fmt.Sprintf("roles=%s", strings.Join(req.GetRoles(), ",")),
				resourceIDsString,
				req.GetCreationTime().Format(time.RFC822),
				time.Until(req.Expiry()).Round(time.Minute).String(),
				time.Until(req.GetAccessExpiry()).Round(time.Minute).String(),
				req.GetState().String(),
				quoteOrDefault(req.GetRequestReason(), ""),
				quoteOrDefault(req.GetResolveReason(), ""),
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
func printRequestsDetailed(reqs []types.AccessRequest, format string) error {
	switch format {
	case teleport.Text:
		for _, req := range reqs {
			resourceIDsString, err := types.ResourceIDsToString(req.GetRequestedResourceIDs())
			if err != nil {
				return trace.Wrap(err)
			}
			if resourceIDsString == "" {
				resourceIDsString = "[none]"
			}
			table := asciitable.MakeHeadlessTable(2)
			table.AddRow([]string{"Token: ", req.GetName()})
			table.AddRow([]string{"Requestor: ", req.GetUser()})
			table.AddRow([]string{"Metadata: ", fmt.Sprintf("roles=%s", strings.Join(req.GetRoles(), ","))})
			table.AddRow([]string{"Resources: ", resourceIDsString})
			table.AddRow([]string{"Created At (UTC): ", req.GetCreationTime().Format(time.RFC822)})
			table.AddRow([]string{"Status: ", req.GetState().String()})
			table.AddRow([]string{"Request Reason: ", quoteOrDefault(req.GetRequestReason(), "[none]")})
			table.AddRow([]string{"Resolve Reason: ", quoteOrDefault(req.GetResolveReason(), "[none]")})

			_, err = table.AsBuffer().WriteTo(os.Stdout)
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

func printJSON(in any, desc string) error {
	out, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return trace.Wrap(err, fmt.Sprintf("failed to marshal %v", desc))
	}
	fmt.Printf("%s\n", out)
	return nil
}

func quoteOrDefault(s, d string) string {
	if s == "" {
		return d
	}
	return fmt.Sprintf("%q", s)
}
