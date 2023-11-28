package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/ui"
)

// sendTeleportInviteReq defines the parameters UI clients must send to send an
// invitation. Note that this explicitly does not match the Cloud API's
// `SendTeleportInvite` as no invite token exists when this API is called
// (because this API creates it, along with the user).
type sendTeleportInviteReq struct {
	// Recipients is a list of emails to invite, each of which will be used as
	// the new user's name and destination for the invite email.
	Recipients []string `json:"recipients"`
	Roles      []string `json:"roles"`
}

// sendTeleportCredentialResetReq is a request from the UI to reset a Teleport
// user's credentials via an emailed link.
type sendTeleportCredentialResetReq struct {
	// Recipient is the user that should be emailed a reset link. The user must
	// already exist.
	Recipient string `json:"recipient"`
}

type userAPIGetter interface {
	GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error)

	CreateUser(ctx context.Context, user types.User) (types.User, error)

	CreateResetPasswordToken(ctx context.Context, req auth.CreateUserTokenRequest) (types.UserToken, error)
}

type cloudAPIGetter interface {
	SendTeleportInvite(ctx context.Context, in *cloudapi.SendTeleportInviteRequest, opts ...grpc.CallOption) (*cloudapi.EmptyResponse, error)
}

func createAndInviteUsers(r *http.Request, authClt userAPIGetter, cloudClt cloudAPIGetter, createdBy string) ([]*ui.User, error) {
	var req sendTeleportInviteReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	var users []*ui.User

	for _, recipient := range req.Recipients {
		// Note: this follows lib/web's createUser() and createResetPasswordToken(),
		// albeit it with some hard-coded values.
		user, err := types.NewUser(recipient)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		user.SetRoles(req.Roles)
		user.SetCreatedBy(types.CreatedBy{
			User: types.UserRef{Name: createdBy},
			Time: time.Now().UTC(),
		})

		if _, err := authClt.CreateUser(r.Context(), user); err != nil {
			return nil, trace.Wrap(err)
		}

		token, err := authClt.CreateResetPasswordToken(r.Context(),
			auth.CreateUserTokenRequest{
				Name: recipient,

				// We'll only ever support password reset tokens here, so we can use
				// the constant.
				Type: auth.UserTokenTypeResetPasswordInvite,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		_, err = cloudClt.SendTeleportInvite(r.Context(), &cloudapi.SendTeleportInviteRequest{
			Recipient: recipient,
			InviteUrl: token.GetURL(),
		})
		if err != nil {
			return nil, trail.FromGRPC(err)
		}

		uiUser, err := ui.NewUser(user)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		users = append(users, uiUser)
	}

	// The UI doesn't particularly care about the return value, and we don't
	// want to distribute the invite token around unnecessarily. Since it's
	// already been emailed to the target user, we'll just return the user
	// object.
	return users, nil
}

// isEmailLike checks if a string could plausibly be a valid email address, specifically that it contains exactly 1 '@'
// character with some text on either side.
func isEmailLike(recipient string) bool {
	parts := strings.Split(recipient, "@")
	if len(parts) != 2 {
		return false
	}

	if parts[0] == "" || parts[1] == "" {
		return false
	}

	return true
}

// ensureValidRecipient makes sure the recipient is a valid Teleport user
// and that the name is email-like.
func ensureValidRecipient(ctx context.Context, authClt userAPIGetter, recipient string) error {
	if !isEmailLike(recipient) {
		return trace.BadParameter("user %q does not have an email-like username", recipient)
	}

	_, err := authClt.GetUser(ctx, recipient, false)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func sendTeleportCredentialResetLink(r *http.Request, authClt userAPIGetter, cloudClt cloudAPIGetter, proxyAddr string) error {
	var req sendTeleportCredentialResetReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}

	// Make sure the recipient is sane before we blindly try emailing a non-email user.s
	err := ensureValidRecipient(r.Context(), authClt, req.Recipient)
	if err != nil {
		return trace.Wrap(err)
	}

	token, err := authClt.CreateResetPasswordToken(r.Context(),
		auth.CreateUserTokenRequest{
			Name: req.Recipient,

			// We'll only ever support password reset tokens here, so we can use
			// the constant.
			Type: auth.UserTokenTypeResetPassword,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = cloudClt.SendTeleportInvite(r.Context(), &cloudapi.SendTeleportInviteRequest{
		Recipient: req.Recipient,
		InviteUrl: token.GetURL(),
	})
	if err != nil {
		return trail.FromGRPC(err)
	}

	return nil
}

func (p *Plugin) sendTeleportInviteHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, cloudClt cloud.Client) (interface{}, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createAndInviteUsers(r, authClt, cloudClt, ctx.GetUser())
}

func (p *Plugin) sendTeleportCredentialResetHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, cloudClt cloud.Client) (interface{}, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = sendTeleportCredentialResetLink(r, authClt, cloudClt, ctx.GetUser())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}
