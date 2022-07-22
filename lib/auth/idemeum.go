package auth

import (
	"context"
	"github.com/gravitational/teleport/lib/idemeumjwt"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func (a *Server) ValidateIdemeumServiceToken(ctx context.Context, ServiceToken string, TenantUrl string) (types.WebSession, error) {
	//validate idemeum token
	log.Info("Validating idemeum service token")
	params, err := validateIdemeumToken(ServiceToken, TenantUrl)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	//create user in teleport
	log.Info("Creating idemeum user in the system")
	user, err := a.createIdemeumUser(params)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	//issue web session only for now
	log.Infof("Issuing web session for user: %v", user.GetName())
	session, err := a.createWebSession(ctx, types.NewWebSessionRequest{
		User:       user.GetName(),
		Roles:      user.GetRoles(),
		Traits:     user.GetTraits(),
		SessionTTL: params.sessionTTL,
		LoginTime:  a.clock.Now().UTC(),
	})

	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("Issued web session for user: %v", user.GetName())
	log.Info("session id:%v, expires: %v", session.GetName(), session.GetExpiryTime())
	return session, nil
}

func validateIdemeumToken(ServiceToken string, TenantUrl string) (*createUserParams, error) {
	if ServiceToken == "" || TenantUrl == "" {
		return nil, trace.BadParameter("missing service token or tenant url")
	}

	claims, err := idemeumjwt.ValidateJwtToken(ServiceToken, TenantUrl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	//For now add the restriction so that normal users and admin cant use their own tokens
	userName := claims.Subject
	if userName == "system" {
		return nil, trace.BadParameter("no `userName` in claims for Idemeum account")
	}

	roles := claims.Roles
	if len(claims.Roles) == 0 {
		roles = []string{"editor", "access", "auditor"}
	}

	var sessionTTL time.Duration
	if claims.SessionTTL == 0 {
		log.Info("Missing sessionTTL setting to default %v nanoseconds", apidefaults.MaxCertDuration)
		sessionTTL = apidefaults.MaxCertDuration
	} else {
		log.Info("SessionTTL was set to %v seconds", claims.SessionTTL)
		sessionTTL = time.Duration(claims.SessionTTL) * time.Second
	}

	p := createUserParams{
		connectorName: constants.Idemeum,
		username:      userName,
		roles:         roles,
		sessionTTL:    utils.MinTTL(sessionTTL, apidefaults.MaxCertDuration),
	}
	return &p, nil
}

func (a *Server) createIdemeumUser(p *createUserParams) (types.User, error) {
	expires := a.GetClock().Now().UTC().Add(p.sessionTTL)

	log.Debugf("Generating dynamic Idemeum identity %v/%v with roles: %v.", p.connectorName, p.username, p.roles)
	user := &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      p.username,
			Namespace: apidefaults.Namespace,
			Expires:   &expires,
		},
		Spec: types.UserSpecV2{
			Roles:  p.roles,
			Traits: p.traits,
			CreatedBy: types.CreatedBy{
				User: types.UserRef{Name: teleport.UserSystem},
				Time: a.clock.Now().UTC(),
				Connector: &types.ConnectorRef{
					Type:     constants.Idemeum,
					ID:       p.connectorName,
					Identity: p.username,
				},
			},
		},
	}

	// Get the user to check if it already exists or not.
	existingUser, err := a.Identity.GetUser(p.username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	ctx := context.TODO()

	// Overwrite existing user if it was created from an external identity provider.
	if existingUser != nil {
		connectorRef := existingUser.GetCreatedBy().Connector

		// If the existing user is a local user, fail and advise how to fix the problem.
		if connectorRef == nil {
			return nil, trace.AlreadyExists("local user with name %q already exists. Either change "+
				"email in Idemeum identity or remove local user and try again.", existingUser.GetName())
		}

		log.Debugf("Overwriting existing user %q created with %v connector %v.",
			existingUser.GetName(), connectorRef.Type, connectorRef.ID)

		if err := a.UpdateUser(ctx, user); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := a.CreateUser(ctx, user); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return user, nil
}
