// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package auth

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	stableunixusersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/stableunixusers/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/utils"
)

// newStableUNIXUsersServiceServer returns a [stableUNIXUsersServiceServer]
// using the given authorizer and server.
func newStableUNIXUsersServiceServer(authorizer authz.Authorizer, server *Server) (*stableUNIXUsersServiceServer, error) {
	uidCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         30 * time.Second,
		Clock:       server.clock,
		Context:     server.closeCtx,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &stableUNIXUsersServiceServer{
		authorizer: authorizer,

		backend:       server.bk,
		readOnlyCache: server.ReadOnlyCache,

		stableUNIXUsers:      server.Services.StableUNIXUsersInternal,
		clusterConfiguration: server.Services.ClusterConfiguration,

		uidCache: uidCache,

		writerSem: make(chan struct{}, 1),
	}, nil
}

// stableUNIXUsersServiceServer is the auth server implementation for the stable
// UNIX users service, including the gRPC interface, authz enforcement, and
// business logic.
type stableUNIXUsersServiceServer struct {
	stableunixusersv1.UnsafeStableUNIXUsersServiceServer

	authorizer authz.Authorizer

	backend       backend.Backend
	readOnlyCache *readonly.Cache

	stableUNIXUsers      services.StableUNIXUsersInternal
	clusterConfiguration services.ClusterConfigurationInternal

	// uidCache caches the fetched or created UIDs for each given username with
	// a short-ish TTL, and combines concurrent requests for the same username.
	uidCache *utils.FnCache

	// writerSem is a 1-buffered channel acting as a semaphore for writes, since
	// concurrent writes would be almost guaranteed to race against each other
	// otherwise.
	writerSem chan struct{}
}

var _ stableunixusersv1.StableUNIXUsersServiceServer = (*stableUNIXUsersServiceServer)(nil)

// ListStableUNIXUsers implements [stableunixusersv1.StableUNIXUsersServiceServer].
func (s *stableUNIXUsersServiceServer) ListStableUNIXUsers(ctx context.Context, req *stableunixusersv1.ListStableUNIXUsersRequest) (*stableunixusersv1.ListStableUNIXUsersResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindStableUNIXUser, types.VerbRead, types.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}

	return s.listStableUNIXUsers(ctx, req)
}

func (s *stableUNIXUsersServiceServer) listStableUNIXUsers(ctx context.Context, req *stableunixusersv1.ListStableUNIXUsersRequest) (*stableunixusersv1.ListStableUNIXUsersResponse, error) {
	users, nextPageToken, err := s.stableUNIXUsers.ListStableUNIXUsers(ctx, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userspb := make([]*stableunixusersv1.StableUNIXUser, 0, len(users))
	for _, user := range users {
		userspb = append(userspb, stableunixusersv1.StableUNIXUser_builder{
			Username: proto.String(user.Username),
			Uid:      proto.Int32(user.UID),
		}.Build())
	}

	return stableunixusersv1.ListStableUNIXUsersResponse_builder{
		StableUnixUsers: userspb,
		NextPageToken:   proto.String(nextPageToken),
	}.Build(), nil
}

// ObtainUIDForUsername implements [stableunixusersv1.StableUNIXUsersServiceServer].
func (s *stableUNIXUsersServiceServer) ObtainUIDForUsername(ctx context.Context, req *stableunixusersv1.ObtainUIDForUsernameRequest) (*stableunixusersv1.ObtainUIDForUsernameResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindStableUNIXUser, types.VerbCreate, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	uid, err := s.obtainUIDForUsernameCached(ctx, req.GetUsername())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return stableunixusersv1.ObtainUIDForUsernameResponse_builder{
		Uid: proto.Int32(uid),
	}.Build(), nil
}

// obtainUIDForUsernameCached calls [obtainUIDForUsernameUncached] through the UID FnCache.
func (s *stableUNIXUsersServiceServer) obtainUIDForUsernameCached(ctx context.Context, username string) (int32, error) {
	uid, err := utils.FnCacheGet(ctx, s.uidCache, username, func(ctx context.Context) (int32, error) {
		return s.obtainUIDForUsernameUncached(ctx, username)
	})
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return uid, nil
}

// obtainUIDForUsernameUncached reads or creates the stable UID for the given username.
func (s *stableUNIXUsersServiceServer) obtainUIDForUsernameUncached(ctx context.Context, username string) (int32, error) {
	if username == "" {
		return 0, trace.BadParameter("username must not be empty")
	}

	// we should only ever race with different auth servers on the same cluster,
	// since we have a semaphore for the local auth
	const maxAttempts = 3
	for attempt := range maxAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return 0, trace.Wrap(context.Cause(ctx))
			case <-time.After(time.Duration(attempt) * 100 * time.Millisecond):
			}
		}

		uid, err := s.stableUNIXUsers.GetUIDForUsername(ctx, username)
		if err == nil {
			// TODO(espadolini): _potentially_ emit an audit log event with
			// username and UID (it might spam the audit log unnecessarily)
			return uid, nil
		}
		if !trace.IsNotFound(err) {
			return 0, trace.Wrap(err)
		}

		var authPref readonly.AuthPreference
		if attempt == 0 {
			ap, err := s.readOnlyCache.GetReadOnlyAuthPreference(ctx)
			if err != nil {
				return 0, trace.Wrap(err)
			}
			authPref = ap
		} else {
			ap, err := s.clusterConfiguration.GetAuthPreference(ctx)
			if err != nil {
				return 0, trace.Wrap(err)
			}
			authPref = ap
		}

		uid, err = s.createNewStableUNIXUser(ctx, username, authPref)
		if err != nil {
			if trace.IsCompareFailed(err) {
				continue
			}
			// if the readOnlyCache is a bit stale we might end up not checking
			// the revision of the cached AuthPreference because the feature
			// might appear to be disabled or the range might be exhausted;
			// since the readOnlyCache is fed by the auth cache, it could
			// potentially be stale for longer than the 1600ms of its internal
			// FnCache, so we can't help but fall back to a backend fetch in
			// that case
			if attempt == 0 && trace.IsLimitExceeded(err) {
				continue
			}
			return 0, trace.Wrap(err)
		}

		// TODO(espadolini): emit an audit log event with the username and UID
		// that was just created

		return uid, nil
	}

	return 0, trace.CompareFailed("exhausted attempts to obtain UID for username")
}

// createNewStableUNIXUser will search the configured UID range for a free UID
// and it will store an entry for the given username with that UID.
func (s *stableUNIXUsersServiceServer) createNewStableUNIXUser(ctx context.Context, username string, authPref readonly.AuthPreference) (int32, error) {
	cfg := authPref.GetStableUNIXUserConfig()
	if cfg == nil || !cfg.Enabled {
		return 0, trace.LimitExceeded("stable UNIX users are not enabled")
	}

	select {
	case s.writerSem <- struct{}{}:
		defer func() { <-s.writerSem }()
	case <-ctx.Done():
		return 0, trace.Wrap(context.Cause(ctx))
	}

	uid, err := s.stableUNIXUsers.SearchFreeUID(ctx, cfg.FirstUid, cfg.LastUid)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	actions, err := s.stableUNIXUsers.AppendCreateStableUNIXUser(nil, username, uid)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	actions, err = s.clusterConfiguration.AppendCheckAuthPreferenceActions(actions, authPref.GetRevision())
	if err != nil {
		return 0, trace.Wrap(err)
	}

	if _, err := s.backend.AtomicWrite(ctx, actions); err != nil {
		return 0, trace.Wrap(err)
	}

	return uid, nil
}
