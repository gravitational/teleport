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

package local

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"math"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/defaults"
	stableunixusersv1 "github.com/gravitational/teleport/gen/proto/go/teleport/storage/local/stableunixusers/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

const (
	stableUNIXUsersPrefix          = "stable_unix_users"
	stableUNIXUsersByUIDInfix      = "by_uid"
	stableUNIXUsersByUsernameInfix = "by_username"
)

// StableUNIXUsersService is the storage service implementation to interact with
// stable UNIX users.
type StableUNIXUsersService struct {
	Backend backend.Backend
}

var _ services.StableUNIXUsersInternal = (*StableUNIXUsersService)(nil)

// GetUIDForUsername implements [services.StableUNIXUsersInternal].
func (s *StableUNIXUsersService) GetUIDForUsername(ctx context.Context, username string) (int32, error) {
	if username == "" {
		return 0, trace.BadParameter("username cannot be empty")
	}

	item, err := s.Backend.Get(ctx, s.usernameToKey(username))
	if err != nil {
		return 0, trace.Wrap(err)
	}

	m := new(stableunixusersv1.StableUNIXUser)
	if err := proto.Unmarshal(item.Value, m); err != nil {
		return 0, trace.Wrap(err)
	}

	return m.GetUid(), nil
}

// ListStableUNIXUsers implements [services.StableUNIXUsersInternal].
func (s *StableUNIXUsersService) ListStableUNIXUsers(ctx context.Context, pageSize int, pageToken string) (_ []services.StableUNIXUser, nextPageToken string, _ error) {
	start := backend.ExactKey(stableUNIXUsersPrefix, stableUNIXUsersByUsernameInfix)
	end := backend.RangeEnd(start)
	if pageToken != "" {
		start = s.usernameToKey(pageToken)
	}

	if pageSize <= 0 {
		pageSize = defaults.DefaultChunkSize
	}

	items, err := s.Backend.GetRange(ctx, start, end, pageSize+1)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	users := make([]services.StableUNIXUser, 0, len(items.Items))
	for _, item := range items.Items {
		userpb := new(stableunixusersv1.StableUNIXUser)
		if err := proto.Unmarshal(item.Value, userpb); err != nil {
			return nil, "", trace.Wrap(err)
		}

		users = append(users, services.StableUNIXUser{
			Username: userpb.GetUsername(),
			UID:      userpb.GetUid(),
		})
	}

	if len(users) > pageSize {
		nextPageToken := users[pageSize].Username
		clear(users[pageSize:])
		return users[:pageSize], nextPageToken, nil
	}

	return users, "", nil
}

// SearchFreeUID implements [services.StableUNIXUsersInternal].
func (s *StableUNIXUsersService) SearchFreeUID(ctx context.Context, first int32, last int32) (int32, bool, error) {
	// uidToKey is monotonic decreasing, so by fetching the key range from last
	// to first we will encounter UIDs in decreasing order
	start := s.uidToKey(last)
	end := s.uidToKey(first)

	// TODO(espadolini): this logic is a big simplification that will only
	// actually return a value in the empty range adjacent to "last", ignoring
	// any free spots before the biggest UID in the range; as an improvement we
	// could occasionally fall back to a full scan and keep track of the last
	// free spot we've encountered in the full scan, which could then be used
	// for later searches (this is something that could be implemented by the
	// caller of SearchFreeUID)

	r, err := s.Backend.GetRange(ctx, start, end, 1)
	if err != nil {
		return 0, false, trace.Wrap(err)
	}

	if len(r.Items) < 1 {
		return first, true, nil
	}

	m := new(stableunixusersv1.StableUNIXUser)
	if err := proto.Unmarshal(r.Items[0].Value, m); err != nil {
		return 0, false, trace.Wrap(err)
	}

	uid := m.GetUid()
	if uid < first || uid > last {
		return 0, false, trace.Errorf("free UID search returned out of range value (this is a bug)")
	}

	if uid == last {
		return 0, false, nil
	}

	return uid + 1, true, nil
}

// AppendCreateUsernameUID implements [services.StableUNIXUsersInternal].
func (s *StableUNIXUsersService) AppendCreateStableUNIXUser(actions []backend.ConditionalAction, username string, uid int32) ([]backend.ConditionalAction, error) {
	b, err := proto.Marshal(&stableunixusersv1.StableUNIXUser{
		Username: username,
		Uid:      uid,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return append(actions,
		backend.ConditionalAction{
			Key:       s.usernameToKey(username),
			Condition: backend.NotExists(),
			Action: backend.Put(backend.Item{
				Value: b,
			}),
		},
		backend.ConditionalAction{
			Key:       s.uidToKey(uid),
			Condition: backend.NotExists(),
			Action: backend.Put(backend.Item{
				Value: b,
			}),
		},
	), nil
}

// usernameToKey returns the key for the "by_username" item with the given
// username. To avoid confusion or encoding problems, the username is
// hex-encoded in the key.
func (*StableUNIXUsersService) usernameToKey(username string) backend.Key {
	suffix := hex.EncodeToString([]byte(username))
	return backend.NewKey(stableUNIXUsersPrefix, stableUNIXUsersByUsernameInfix, suffix)
}

// uidToKey returns the key for the "by_uid" item with the given UID. The
// resulting keys have the opposite order to the numbers, such that the key for
// [math.MaxInt32] is the smallest and the key for [math.MinInt32] is the
// largest.
func (*StableUNIXUsersService) uidToKey(uid int32) backend.Key {
	// in two's complement (which Go specifies), this transformation maps
	// +0x7fff_ffff (MaxInt32) to 0x0000_0000 and -0x8000_0000 (MinInt32) to
	// 0xffff_ffff; as such, GetRange will return items from the largest to the
	// smallest, which we rely on to get the largest allocated item in the range
	// we're interested in
	suffix := hex.EncodeToString(binary.BigEndian.AppendUint32(nil, math.MaxInt32-uint32(uid)))
	return backend.NewKey(stableUNIXUsersPrefix, stableUNIXUsersByUIDInfix, suffix)
}
