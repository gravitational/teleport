/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalsecrets/v1"
)

const (
	// DefaultMaxSecretsPerSession is the maximum number of secrets a single
	// session credential resource can hold.
	DefaultMaxSecretsPerSession = 10
)

// UserSessionCredentials is the interface for managing per-user per-session
// credentials resources.
type UserSessionCredentials interface {
	Get(ctx context.Context, user, keyID string) (*pb.UserSessionCredentials, error)
	GetByKeyID(ctx context.Context, user, keyID string) (*pb.UserSessionCredentials, error)
	GetByDelegation(ctx context.Context, user, delegationSessionID, botInstanceID string) (*pb.UserSessionCredentials, error)
	Create(ctx context.Context, resource *pb.UserSessionCredentials) (*pb.UserSessionCredentials, error)
	Put(ctx context.Context, resource *pb.UserSessionCredentials) (*pb.UserSessionCredentials, error)
	Update(ctx context.Context, resource *pb.UserSessionCredentials) (*pb.UserSessionCredentials, error)
	Delete(ctx context.Context, user, keyID string) error
	DeleteByDelegation(ctx context.Context, user, delegationSessionID, botInstanceID string) error
	DeleteByDelegationSession(ctx context.Context, user, delegationSessionID string) error
	ListByUser(ctx context.Context, user string) ([]*pb.UserSessionCredentials, error)
}
