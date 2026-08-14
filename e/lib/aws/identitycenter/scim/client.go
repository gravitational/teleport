package scim

import (
	"context"
	"log/slog"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/aws/identitycenter/provisioning"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
)

// client wraps a SCIM client, overriding the underlying client methods to provide
// Identity-Center specific behavior
type client struct {
	scimsdk.Client
	icClient    icsdk.Client
	provisioner *provisioning.AssignmentProvisioner
	log         *slog.Logger
}

// ClientConfig provides the configuration for a new Identity Center SCIM client
type ClientConfig struct {
	SCIMClient  scimsdk.Client
	APIClient   icsdk.Client
	Provisioner *provisioning.AssignmentProvisioner
	Log         *slog.Logger
}

func (cfg *ClientConfig) CheckAndSetDefaults() error {
	if cfg.SCIMClient == nil {
		return trace.BadParameter("missing SCIM client")
	}

	if cfg.APIClient == nil {
		return trace.BadParameter("missing IC API client")
	}

	if cfg.Provisioner == nil {
		return trace.BadParameter("missing Account Assignment provisioner")
	}

	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	return nil
}

// NewClient creates a SCIM client wrapper that overrides the default SCIM
// client in order to add Identity-Center specific behavior.
func NewClient(cfg ClientConfig) (scimsdk.Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	client := &client{
		Client:      cfg.SCIMClient,
		icClient:    cfg.APIClient,
		provisioner: cfg.Provisioner,
		log:         cfg.Log,
	}
	return client, nil
}

// ListGroupMembers uses the AWS Identity Center API to list the current members
// of a group. This provides accurate membership data that is not available via
// the SCIM API.
func (c *client) ListGroupMembers(ctx context.Context, id string) ([]*scimsdk.GroupMember, error) {
	icMembers, err := c.icClient.ListGroupMemberships(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members := make([]*scimsdk.GroupMember, len(icMembers))
	for i, m := range icMembers {
		members[i] = &scimsdk.GroupMember{
			ExternalID: m.MemberID,
			Type:       scimsdk.ResourceTypeUser,
		}
	}
	return members, nil
}

// DeleteGroup overrides the default [scimsdk.Client.DeleteGroup] implementation
// in order to try and delete the target group's account assignments before
// deleting the group itself.
//
// AWS Identity Center should technically do this for us when we delete a group
// via SCIM, but unfortunately it sometimes leaves behind "orphan assignments"
// that must be manually deleted by the AWS admin. To be a good citizen we make
// a best-effort attempt to clean up the account assignments before we delete
// the group, but any failures in deleting the Account Assignments will not stop
// the Group deletion.
func (c *client) DeleteGroup(ctx context.Context, id string) error {
	log := c.log.With("group_id", id)

	// Make a best-effort attempt to delete the group's account assignments
	// before passing the group delete down to the SCIM handler. Identity Center
	// doesn't always clean these up for us like it should.
	log.InfoContext(ctx, "Deleting group account assignments before deprovisioning")
	if err := c.provisioner.DeleteAllAssignments(ctx, id, ssoadmintypes.PrincipalTypeGroup); err != nil {
		log.WarnContext(ctx, "Error deleting group account assignments", "error", err)
	}

	return trace.Wrap(c.Client.DeleteGroup(ctx, id))
}

// DeleteUser overrides the default [scimsdk.Client.DeleteUser] implementation
// in order to try and delete the target user's account assignments before
// deleting the user itself.
//
// AWS Identity Center should technically do this for us when we delete a user
// via SCIM, but unfortunately it sometimes leaves behind "orphan assignments"
// in the AWS console that must be manually deleted by the AWS admin. To be a
// good citizen we make a best-effort attempt to clean up the account assignments
// before we delete the group, but any failures in deleting the Account
// Assignments will not stop the Group deletion.
func (c *client) DeleteUser(ctx context.Context, id string) error {
	log := c.log.With("user_id", id)

	// Make a best-effort attempt to delete the user's account assignments
	// before passing the delete down to the SCIM handler. Identity Center
	// doesn't always clean these up for us like it should.
	log.InfoContext(ctx, "Deleting user account assignments before deprovisioning")
	if err := c.provisioner.DeleteAllAssignments(ctx, id, ssoadmintypes.PrincipalTypeUser); err != nil {
		log.WarnContext(ctx, "Error deleting user account assignments", "error", err)
	}

	return trace.Wrap(c.Client.DeleteUser(ctx, id))
}
