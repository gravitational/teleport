// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"crypto/tls"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport/api/types"
)

// SessionAuth is a convenience wrapper around Auth interface that uses Session type instead of fine-grained parameters.
type SessionAuth interface {
	// GetRDSAuthToken generates RDS/Aurora auth token.
	GetRDSAuthToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetRedshiftAuthToken generates Redshift auth token.
	GetRedshiftAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error)
	// GetRedshiftServerlessAuthToken generates Redshift Serverless auth token.
	GetRedshiftServerlessAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error)
	// GetElastiCacheRedisToken generates an ElastiCache Redis auth token.
	GetElastiCacheRedisToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetMemoryDBToken generates a MemoryDB auth token.
	GetMemoryDBToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetCloudSQLAuthToken generates Cloud SQL auth token.
	GetCloudSQLAuthToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetSpannerTokenSource returns an oauth token source for GCP Spanner.
	GetSpannerTokenSource(ctx context.Context, sessionCtx *Session) (oauth2.TokenSource, error)
	// GetCloudSQLPassword generates password for a Cloud SQL database user.
	GetCloudSQLPassword(ctx context.Context, sessionCtx *Session) (string, error)
	// GetAzureAccessToken generates Azure database access token.
	GetAzureAccessToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetAzureCacheForRedisToken retrieves auth token for Azure Cache for Redis.
	GetAzureCacheForRedisToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetTLSConfig builds the client TLS configuration for the session.
	GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error)
	// GetAuthPreference returns the cluster authentication config.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
	// GetAzureIdentityResourceID returns the Azure identity resource ID
	// attached to the current compute instance. If Teleport is not running on
	// Azure VM returns an error.
	GetAzureIdentityResourceID(ctx context.Context, identityName string) (string, error)
	// GetAWSIAMCreds returns the AWS IAM credentials, including access key,
	// secret access key and session token.
	GetAWSIAMCreds(ctx context.Context, sessionCtx *Session) (string, string, string, error)
	// Closer releases all resources used by authenticator.
	// io.Closer
}

type sessionAuth struct {
	auth  Auth
	clock clockwork.Clock
}

func NewSessionAuth(auth Auth, clock clockwork.Clock, sessionCtx *Session) SessionAuth {
	return &sessionAuth{
		auth: auth.WithLogger(func(logger logrus.FieldLogger) logrus.FieldLogger {
			return logger.WithFields(logrus.Fields{
				"session_id": sessionCtx.ID,
				"database":   sessionCtx.Database.GetName(),
			})
		}),
		clock: clock,
	}
}

func (s *sessionAuth) GetRDSAuthToken(ctx context.Context, sessionCtx *Session) (string, error) {
	return s.auth.GetRDSAuthToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser)
}

func (s *sessionAuth) GetRedshiftAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error) {
	return s.auth.GetRedshiftAuthToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser, sessionCtx.DatabaseName)
}

func (s *sessionAuth) GetRedshiftServerlessAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error) {
	return s.auth.GetRedshiftServerlessAuthToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser, sessionCtx.DatabaseName)
}

func (s *sessionAuth) GetElastiCacheRedisToken(ctx context.Context, sessionCtx *Session) (string, error) {
	return s.auth.GetElastiCacheRedisToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser, sessionCtx.DatabaseName)
}

func (s *sessionAuth) GetMemoryDBToken(ctx context.Context, sessionCtx *Session) (string, error) {
	return s.auth.GetMemoryDBToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser, sessionCtx.DatabaseName)
}

func (s *sessionAuth) GetCloudSQLAuthToken(ctx context.Context, sessionCtx *Session) (string, error) {
	return s.auth.GetCloudSQLAuthToken(ctx, sessionCtx.DatabaseUser)
}

func (s *sessionAuth) GetSpannerTokenSource(ctx context.Context, sessionCtx *Session) (oauth2.TokenSource, error) {
	return s.auth.GetSpannerTokenSource(ctx, sessionCtx.DatabaseUser)
}

func (s *sessionAuth) GetCloudSQLPassword(ctx context.Context, sessionCtx *Session) (string, error) {
	return s.auth.GetCloudSQLPassword(ctx, sessionCtx.Database, sessionCtx.DatabaseUser, sessionCtx.DatabaseName)
}

func (s *sessionAuth) GetAzureAccessToken(ctx context.Context, sessionCtx *Session) (string, error) {
	return s.auth.GetAzureAccessToken(ctx, sessionCtx.Database, sessionCtx.DatabaseUser, sessionCtx.DatabaseName)
}

func (s *sessionAuth) GetAzureCacheForRedisToken(ctx context.Context, sessionCtx *Session) (string, error) {
	return s.auth.GetAzureCacheForRedisToken(ctx, sessionCtx.Database)
}

func (s *sessionAuth) GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	ttl := sessionCtx.Identity.Expires.Sub(s.clock.Now())
	return s.auth.GetTLSConfig(ctx, ttl, sessionCtx.Database, sessionCtx.DatabaseUser, sessionCtx.DatabaseName)
}

func (s *sessionAuth) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return s.auth.GetAuthPreference(ctx)
}

func (s *sessionAuth) GetAzureIdentityResourceID(ctx context.Context, identityName string) (string, error) {
	return s.auth.GetAzureIdentityResourceID(ctx, identityName)
}

func (s *sessionAuth) GetAWSIAMCreds(ctx context.Context, sessionCtx *Session) (string, string, string, error) {
	return s.auth.GetAWSIAMCreds(ctx, sessionCtx.Database, sessionCtx.DatabaseUser)
}

type reportingSessionAuth struct {
	SessionAuth
	component string
	db        types.Database
}

// newReportingSessionAuth returns a reporting version of Auth, wrapping the original Auth instance.
func newReportingSessionAuth(db types.Database, auth SessionAuth) SessionAuth {
	return &reportingSessionAuth{
		SessionAuth: auth,
		component:   "db:auth",
		db:          db,
	}
}

func (r *reportingSessionAuth) GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	defer methodCallMetrics("GetTLSConfig", r.component, r.db)()
	return r.SessionAuth.GetTLSConfig(ctx, sessionCtx)
}
