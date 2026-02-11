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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
)

const (
	// ToolsetAccessListReviews is the name of the category of MCP tools related
	// to performing access list reviews.
	ToolsetAccessListReviews = "access-list-reviews"
	// ToolAccessListReviewInstructions is the name of the MCP tool that
	// provides access list review instructions.
	ToolAccessListReviewInstuctions = "teleport-access-list-review-instructions"
	// ToolAccessListsForReview is the name of the MCP tool that fetches
	// access lists requiring review.
	ToolAccessListsForReview = "teleport-get-access-lists-for-review"
	// ToolReviewAccessList is the name of the MCP tool that submits access list
	// reviews.
	ToolReviewAccessList = "teleport-review-access-list"
)

func init() {
	RegisterTool(ToolsetAccessListReviews, NewAccessListReviewInstructionsTool)
	RegisterTool(ToolsetAccessListReviews, NewAccessListsForReviewTool)
	RegisterTool(ToolsetAccessListReviews, NewReviewAccessListTool)
}

// NewAccessListReviewInstructionsTool creates an MCP tool that returns
// access list review instructions.
func NewAccessListReviewInstructionsTool(cfg Config) (Tool, error) {
	return &accessListReviewInstructionsTool{cfg}, nil
}

type accessListReviewInstructionsTool struct {
	cfg Config
}

func (s *accessListReviewInstructionsTool) GetTool() mcp.Tool {
	return mcp.NewTool(ToolAccessListReviewInstuctions,
		mcp.WithDescription(`Returns analysis instructions for access list
reviews. Call this tool when asked to view or review Teleport access lists and
then follow the returned instructions.`))
}

func (s *accessListReviewInstructionsTool) GetHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(s.getAccessListReviewInstructions()), nil
	}
}

func (s *accessListReviewInstructionsTool) getAccessListReviewInstructions() string {
	return accessListReviewInstructions
}

// NewAccessListsForReviewTool creates an MCP tool that fetches access lists
// requiring review.
func NewAccessListsForReviewTool(cfg Config) (Tool, error) {
	return &accessListsForReviewTool{cfg}, nil
}

type accessListsForReviewTool struct {
	cfg Config
}

func (s *accessListsForReviewTool) GetTool() mcp.Tool {
	return mcp.NewTool(ToolAccessListsForReview,
		mcp.WithDescription(`Fetch Teleport access lists requiring review,
along with their details including members, owner/member roles and audit
history.`))
}

func (s *accessListsForReviewTool) GetHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		lists, err := s.getAccessListsToReview(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		b, err := json.Marshal(lists)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

// accessListDetails contains information about an access list that an LLM
// can use to analyze it for review purposes.
//
// We avoid returning full yaml resources to eliminate insignificant fields
// that would just add noise to the LLM context.
type accessListDetails struct {
	Name        string
	Description string
	Labels      map[string]string
	Title       string
	Owners      []accesslist.Owner
	Audit       accesslist.Audit
	MemberRoles []*types.RoleV6
	Members     []*accesslist.AccessListMember
	Reviews     []*accesslist.Review
	URL         string
}

func (s *accessListsForReviewTool) getAccessListsToReview(ctx context.Context) ([]accessListDetails, error) {
	lists, err := s.cfg.Auth.AccessListClient().GetAccessListsToReview(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result []accessListDetails
	for _, list := range lists {
		details, err := s.getAccessListDetails(ctx, list.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, *details)
	}
	return result, nil
}

func (s *accessListsForReviewTool) getAccessListDetails(ctx context.Context, name string) (*accessListDetails, error) {
	list, err := s.cfg.Auth.AccessListClient().GetAccessList(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberRoles, err := s.getAccessListRoles(ctx, list)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(r0mant): 200 members per list max should be fine for now, but we
	// might want to paginate in the future.
	members, _, err := s.cfg.Auth.AccessListClient().ListAccessListMembers(ctx, name, 200, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(r0mant): similar to the above, 100 reviews should be plenty for now.
	reviews, _, err := s.cfg.Auth.AccessListClient().ListAccessListReviews(ctx, name, 100, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	url := ""
	if s.cfg.WebProxyAddr != "" {
		url = fmt.Sprintf(accessListURLTemplate, s.cfg.WebProxyAddr, name)
	}

	return &accessListDetails{
		Name:        list.GetName(),
		Description: list.GetMetadata().Description,
		Labels:      list.GetMetadata().Labels,
		Title:       list.Spec.Title,
		Owners:      list.GetOwners(),
		Audit:       list.Spec.Audit,
		MemberRoles: memberRoles,
		Members:     members,
		Reviews:     reviews,
		URL:         url,
	}, nil
}

func (s *accessListsForReviewTool) getAccessListRoles(ctx context.Context, list *accesslist.AccessList) ([]*types.RoleV6, error) {
	var memberRoles []*types.RoleV6
	for _, roleName := range list.GetGrants().Roles {
		role, err := s.cfg.Auth.GetRole(ctx, roleName)
		if err != nil && !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}
		// Owners may not have permissions to read roles.
		if trace.IsAccessDenied(err) {
			continue
		}
		roleV6, ok := role.(*types.RoleV6)
		if !ok {
			return nil, trace.BadParameter("expected role %q to be RoleV6, got %T", roleName, role)
		}
		memberRoles = append(memberRoles, roleV6)
	}
	return memberRoles, nil
}

// NewReviewAccessListTool creates an MCP tool that submits access list review.
func NewReviewAccessListTool(cfg Config) (Tool, error) {
	return &reviewAccessListTool{cfg}, nil
}

type reviewAccessListTool struct {
	cfg Config
}

func (s *reviewAccessListTool) GetTool() mcp.Tool {
	return mcp.NewTool(ToolReviewAccessList,
		mcp.WithDescription(`Submit Teleport access lists review.`),
		mcp.WithString("name", mcp.Required()))
}

func (s *reviewAccessListTool) GetHandler() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		if name == "" {
			return nil, trace.BadParameter("missing MCP parameter 'name'")
		}
		updatedReview, err := s.reviewAccessList(ctx, name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		b, err := json.Marshal(updatedReview)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return mcp.NewToolResultText(string(b)), nil
	}
}

func (s *reviewAccessListTool) reviewAccessList(ctx context.Context, name string) (*accesslist.Review, error) {
	review, err := accesslist.NewReview(header.Metadata{
		Name: uuid.NewString(),
	}, accesslist.ReviewSpec{
		AccessList: name,
		ReviewDate: time.Now(),
		Reviewers:  []string{"mcp"}, // Required by API but does not matter as it'll be set server-side to user identity.
		Notes:      "Reviewed via MCP",
		Changes:    accesslist.ReviewChanges{},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updatedReview, _, err := s.cfg.Auth.AccessListClient().CreateAccessListReview(ctx, review)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updatedReview, nil
}

const accessListURLTemplate = "https://%s/web/accesslists/%s"
const accessListReviewInstructions = `You are helping review Teleport access lists. Follow this workflow:

Step 1: Fetch access lists requiring review

Call the "teleport-get-access-lists-for-review" tool to get access lists
that require review within the time interval specified by the user. If the
time interval is not specified, default to the next 2 weeks.

Step 2: Analyze access lists

For each access list, analyze its risk level to determine whether it can be
auto-recertified or requires human attention. Consider factors such as:

- List name, title, and description
- Permissions granted to list members via roles, if available
- Members and owners of the list
- Past audits and their outcomes

Step 3: Present findings

Display a table categorizing access lists into low-risk lists that can be
auto-recertified and those that may require human attention. Include the
following columns in the table:

- List title
- Review due date
- Auto-review (✅ or ❌)
- Risk assessment (concise 1-2 sentence explanation of the decision)

Step 4: Ask for user confirmation

Ask the user if they would like to proceed with auto-reviewing all low-risk
access lists. If the user confirms:

- Submit reviews for low-risk lists using the "teleport-review-access-list" tool.
- Display a table with URLs for lists that require human review in Teleport with
  brief justifications on what to pay attention to for each.
`
