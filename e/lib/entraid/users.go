package entraid

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/microsoftgraph/msgraph-sdk-go/models"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type entraUniqueID string

func (r *DirectoryReconciler) reconcileUsers(ctx context.Context) (map[entraUniqueID]types.User, error) {
	teleportUsers, err := listTeleportUsers(ctx, r.userSvc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	entraUsers, err := listEntraUsers(ctx, r.graphClient, r.tenantID, r.ssoConnectorID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, src := range teleportUsers {
		if dst, ok := entraUsers[src.GetName()]; ok {
			preserveUserMetadata(dst, src)
		}
	}

	usersByEntraID := map[entraUniqueID]types.User{}
	for _, u := range entraUsers {
		id, ok := u.GetLabel(types.EntraUniqueIDLabel)
		if !ok {
			return nil, trace.BadParameter("user %v missing Entra ID unique ID label", u.GetName())
		}
		usersByEntraID[entraUniqueID(id)] = u
	}

	backend, err := services.NewReconciler(services.ReconcilerConfig[types.User]{
		Matcher:             matchByLabel[types.User],
		GetCurrentResources: func() map[string]types.User { return teleportUsers },
		GetNewResources:     func() map[string]types.User { return entraUsers },
		OnCreate: func(ctx context.Context, u types.User) error {
			_, err := r.userSvc.CreateUser(ctx, u)
			// if Entra user clashes with a local user, do not overwrite
			if trace.IsAlreadyExists(err) {
				slog.InfoContext(ctx, "user already exists in teleport as a non-entra user, not overwriting", "user", u)

				// Delete from the lookup map, since the Teleport user by this name is not an Entra user.
				delete(usersByEntraID, entraUniqueID(u.GetMetadata().Labels[types.EntraUniqueIDLabel]))

				return nil
			}
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming types.User, existing types.User) error {
			_, err := r.userSvc.UpdateUser(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, u types.User) error {
			err := r.userSvc.DeleteUser(ctx, u.GetName())
			return trace.Wrap(err)
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := backend.Reconcile(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	r.importedUsers = len(usersByEntraID)
	return usersByEntraID, nil
}

func listTeleportUsers(ctx context.Context, svc userAccessPoint) (map[string]types.User, error) {
	result := map[string]types.User{}

	var pageToken string
	for {
		resp, err := svc.ListUsers(ctx, &userspb.ListUsersRequest{PageToken: pageToken})
		if err != nil {
			return nil, trace.Wrap(err, "listing teleport entra users")
		}

		for _, user := range resp.Users {
			if matchByLabel(user) {
				result[user.GetName()] = user
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return result, nil
}

func listEntraUsers(ctx context.Context, graphClient graphClient, tenantID string, ssoConnectorID string) (map[string]types.User, error) {
	result := map[string]types.User{}
	err := graphClient.IterateUsers(ctx, func(u models.Userable) bool {
		user, err := convertUser(u, tenantID, ssoConnectorID)
		if err == nil {
			result[user.GetName()] = user
		} else {
			slog.ErrorContext(ctx, "failed to convert Entra ID user to Teleport user", "error", err)
		}
		return true
	})

	return result, trace.Wrap(err)
}

func convertUser(in models.Userable, tenantID string, ssoConnectorID string) (types.User, error) {
	upn := in.GetUserPrincipalName()
	if upn == nil {
		return nil, trace.BadParameter("expected Entra ID user to have a UPN")
	}

	username := in.GetMail()
	if username == nil {
		username = upn
	}

	samAccountName := in.GetOnPremisesSamAccountName()

	out, err := types.NewUser(*username)
	labels := map[string]string{
		types.EntraUniqueIDLabel: *in.GetId(),
		types.EntraTenantIDLabel: tenantID,
		types.EntraUPNLabel:      *upn,
	}
	if samAccountName != nil {
		labels[types.EntraSAMAccountNameLabel] = *samAccountName
	}
	out.SetStaticLabels(labels)
	out.SetOrigin(types.OriginEntraID)
	// Explicitly set to UNSET for idempotency.
	out.SetPasswordState(types.PasswordState_PASSWORD_STATE_UNSET)

	out.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{
			Name: teleport.UserSystem,
		},
		Time: time.Now().UTC(),
		Connector: &types.ConnectorRef{
			ID:       ssoConnectorID,
			Type:     constants.SAML,
			Identity: *upn,
		},
	})

	return out, trace.Wrap(err)
}

// preserveUserMetadata copies any metadata that needs to be preserved across an
// update from src to dst.
func preserveUserMetadata(dst, src types.User) {
	dst.SetRevision(src.GetRevision())
	dst.SetCreatedBy(src.GetCreatedBy())
}
