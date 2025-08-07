package auth

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/session"
)

func NewSessionRecordingAuthorized(authServer *Server, authorizer authz.Authorizer) *SessionRecordingAuthorized {
	return &SessionRecordingAuthorized{
		authServer: authServer,
		authorizer: authorizer,
	}
}

type SessionRecordingAuthorized struct {
	authServer *Server
	authorizer authz.Authorizer
}

func (a *SessionRecordingAuthorized) Authorize(ctx context.Context, sessionID string) error {
	userCtx, err := a.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	serverWithRoles := &ServerWithRoles{
		authServer: a.authServer,
		alog:       a.authServer,
		context:    *userCtx,
	}

	return trace.Wrap(serverWithRoles.actionForKindSession(ctx, session.ID(sessionID)))
}
