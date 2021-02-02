/*
Copyright 2020 Gravitational, Inc.

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

package local

import (
	"context"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// GetAppSession gets an application web session.
func (s *IdentityService) GetAppSession(ctx context.Context, req services.GetAppSessionRequest) (services.WebSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	item, err := s.Get(ctx, backend.Key(appsPrefix, sessionsPrefix, req.SessionID))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := services.UnmarshalWebSession(item.Value, services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return session, nil
}

// GetAppSessions gets all application web sessions.
func (s *IdentityService) GetAppSessions(ctx context.Context) ([]services.WebSession, error) {
	startKey := backend.Key(appsPrefix, sessionsPrefix)
	result, err := s.GetRange(ctx, startKey, backend.RangeEnd(startKey), backend.NoLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]services.WebSession, len(result.Items))
	for i, item := range result.Items {
		session, err := services.UnmarshalWebSession(item.Value, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = session
	}
	return out, nil
}

// UpsertAppSession creates an application web session.
func (s *IdentityService) UpsertAppSession(ctx context.Context, session services.WebSession) error {
	value, err := services.MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:     backend.Key(appsPrefix, sessionsPrefix, session.GetName()),
		Value:   value,
		Expires: session.GetExpiryTime(),
	}

	if _, err = s.Put(ctx, item); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAppSession removes an application web session.
func (s *IdentityService) DeleteAppSession(ctx context.Context, req services.DeleteAppSessionRequest) error {
	if err := s.Delete(ctx, backend.Key(appsPrefix, sessionsPrefix, req.SessionID)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllAppSessions removes all application web sessions.
func (s *IdentityService) DeleteAllAppSessions(ctx context.Context) error {
	startKey := backend.Key(appsPrefix, sessionsPrefix)
	if err := s.DeleteRange(ctx, startKey, backend.RangeEnd(startKey)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
