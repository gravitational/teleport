package msgraph

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"path"
	"strconv"

	"github.com/gravitational/trace"
)

// iterateSimple implements pagination for "simple" object lists, where additional logic isn't needed
func iterateSimple[T any](c *client, ctx context.Context, endpoint string, f func(*T) bool) error {
	var err error
	itErr := c.iterate(ctx, endpoint, func(msg json.RawMessage) bool {
		var page []T
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		for _, item := range page {
			if !f(&item) {
				return false
			}
		}
		return true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}

func (c *client) iterate(ctx context.Context, endpoint string, f func(json.RawMessage) bool) error {
	uri := *c.baseURL
	uri.Path = path.Join(uri.Path, endpoint)
	uri.RawQuery = url.Values{"$top": {strconv.Itoa(c.pageSize)}}.Encode()
	uriString := uri.String()
	for uriString != "" {
		resp, err := c.get(ctx, uriString)
		if err != nil {
			return trace.Wrap(err)
		}
		defer resp.Body.Close()

		var page oDataPage
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return trace.Wrap(err)
		}
		uriString = page.NextLink
		if !f(page.Value) {
			break
		}
	}

	return nil
}

// IterateApplications implements Client.
func (c *client) IterateApplications(ctx context.Context, f func(*Application) bool) error {
	return iterateSimple(c, ctx, "applications", f)
}

// IterateGroups implements Client.
func (c *client) IterateGroups(ctx context.Context, f func(*Group) bool) error {
	return iterateSimple(c, ctx, "groups", f)
}

// IterateUsers implements Client.
func (c *client) IterateUsers(ctx context.Context, f func(*User) bool) error {
	return iterateSimple(c, ctx, "users", f)
}

// IterateGroupMembers implements Client.
func (c *client) IterateGroupMembers(ctx context.Context, groupID string, f func(GroupMember) bool) error {
	var err error
	itErr := c.iterate(ctx, path.Join("groups", groupID, "members"), func(msg json.RawMessage) bool {
		var page []json.RawMessage
		if err = json.Unmarshal(msg, &page); err != nil {
			return false
		}
		for _, entry := range page {
			var member GroupMember
			member, err = decodeGroupMember(entry)
			if err != nil {
				var gmErr *UnsupportedGroupMember
				if errors.As(err, &gmErr) {
					slog.Debug("unsupported group member", "type", gmErr.Type)
					err = nil // Reset so that we do not return the error up if this is the last entry
					continue
				} else {
					return false
				}
			}
			if !f(member) {
				return false
			}
		}
		return true
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(itErr)
}
