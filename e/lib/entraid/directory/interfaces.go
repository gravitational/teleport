package directory

import (
	"context"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/accesslists"
)

// accessPoint to be used by the service.
type accessPoint interface {
	userAccessPoint
	accessListAccessPoint
	connectorAccessPoint
}

type userAccessPoint interface {
	CreateUser(ctx context.Context, user types.User) (types.User, error)
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	DeleteUser(ctx context.Context, user string) error
	UpdateUser(ctx context.Context, user types.User) (types.User, error)
}

type accessListAccessPoint interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	DeleteAccessList(context.Context, string) error

	UpsertAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, membersIn []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error)

	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
	UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error)
	DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error
	InsertAccessListCollection(ctx context.Context, collection *accesslists.Collection) error
}

type connectorAccessPoint interface {
	GetSAMLConnector(ctx context.Context, name string, withSecrets bool) (types.SAMLConnector, error)
}
