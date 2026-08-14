package client

import (
	"context"
	"slices"

	"github.com/gravitational/trace"
)

// User represents a user in the Identity Vault.
type User struct {
	// DN is the distinguished name of the user.
	DN string
	// Email is the email address of the user.
	Email string
	// FullName is the full name of the user.
	FullName string
	// IsDisabled is a flag that determines whether the user is disabled.
	IsDisabled bool
}

// ListUsers returns a list of all users in the Identity Vault.
func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	const usersBase = "rest/access/users/list"

	users, err := listResponse(
		ctx,
		c,
		usersBase,
		func(r listUsersResponse) ([]User, error) {
			var users []User
			for _, u := range r.UsersList {
				isDisabled := slices.Contains(u.DisabledLogin, "true")
				email := ""
				for _, sa := range u.SecondaryAttributes {
					const emailKey = "Email"
					if sa.Key == emailKey && len(sa.AttributeValues) > 0 {
						email = sa.AttributeValues[0].Value
						break
					}
				}

				users = append(users,
					User{
						DN:         u.Dn,
						FullName:   u.FullName,
						IsDisabled: isDisabled,
						Email:      email,
					},
				)
			}
			return users, nil
		},
		withQueryParams("q", "*"),
	)

	return users, trace.Wrap(err)
}

type listUsersResponse struct {
	UsersList []struct {
		Dn                  string `json:"dn"`
		FullName            string `json:"fullName"`
		SecondaryAttributes []struct {
			Key             string `json:"key"`
			DisplayLabel    string `json:"displayLabel"`
			DataType        string `json:"dataType"`
			AttributeValues []struct {
				Type  string `json:"@type"`
				Value string `json:"$"`
			} `json:"attributeValues,omitempty"`
		} `json:"secondaryAttributes"`
		DisabledLogin []string `json:"disabledLogin"`
	} `json:"usersList"`
	NextIndex int `json:"nextIndex"`
}

func (n listUsersResponse) GetNextIndex() int {
	return n.NextIndex
}
