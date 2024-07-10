package msgraph

import "context"

// TODO: split into smaller interfaces
type Client interface {
	IterateUsers(ctx context.Context, f func(*User) bool) error
	IterateGroups(ctx context.Context, f func(*Group) bool) error
	IterateGroupMembers(ctx context.Context, groupID string, f func(GroupMember) bool) error
	IterateApplications(ctx context.Context, f func(*Application) bool) error

	CreateFederatedIdentityCredential(ctx context.Context, appObjectID string, cred *FederatedIdentityCredential) error
	CreateServicePrincipalTokenSigningCertificate(ctx context.Context, spID string, displayName string) (*SelfSignedCertificate, error)
	GetServicePrincipalsByDisplayName(ctx context.Context, displayName string) ([]*ServicePrincipal, error)
	GetServicePrincipalsByAppId(ctx context.Context, appID string) ([]*ServicePrincipal, error)
	InstantiateApplicationTemplate(ctx context.Context, appTemplateID string, displayName string) (*ApplicationServicePrincipal, error)
}
