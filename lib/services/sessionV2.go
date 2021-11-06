/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"context"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// SessionV2 is a realtime session service that has information about
// sessions that are in-flight in the cluster at the moment.
type SessionV2 interface {
	GetActiveSessions(ctx context.Context) ([]types.Session, error)
	CreateSession(ctx context.Context, req *proto.CreateSessionRequest) (types.Session, error)
	UpdateSession(ctx context.Context, req *proto.UpdateSessionRequest) error
	RemoveSession(ctx context.Context, sessionID string) error
}

type sessionV2 struct {
	bk backend.Backend
}

func NewSessionV2Service(bk backend.Backend) SessionV2 {
	return &sessionV2{bk}
}

func (s *sessionV2) GetActiveSessions(ctx context.Context) ([]types.Session, error) {
	panic("unimplemented")
}

func (s *sessionV2) CreateSession(ctx context.Context, req *proto.CreateSessionRequest) (types.Session, error) {
	panic("unimplemented")
}

func (s *sessionV2) UpdateSession(ctx context.Context, req *proto.UpdateSessionRequest) error {
	panic("unimplemented")
}

func (s *sessionV2) RemoveSession(ctx context.Context, sessionID string) error {
	panic("unimplemented")
}
