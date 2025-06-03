package collections

import (
	"fmt"
	"io"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

type userCollection struct {
	users []types.User
}

func NewUserCollection(users []types.User) ResourceCollection {
	return &userCollection{users: users}
}

func (u *userCollection) Resources() (r []types.Resource) {
	for _, resource := range u.users {
		r = append(r, resource)
	}
	return r
}

func (u *userCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"User"})
	for _, user := range u.users {
		t.AddRow([]string{user.GetName()})
	}
	fmt.Println(t.AsBuffer().String())
	return nil
}
