// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package debugv1

import (
	"context"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// fakeChecker is a minimal services.AccessChecker for testing authorization.
type fakeChecker struct {
	allowedVerbs []string
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindDebugService {
		if slices.Contains(f.allowedVerbs, verb) {
			return nil
		}
	}
	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

func newTestService(t *testing.T, allowedVerbs []string) *Service {
	t.Helper()
	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("test-user")
		if err != nil {
			return nil, err
		}
		return &authz.Context{
			User:    user,
			Checker: fakeChecker{allowedVerbs: allowedVerbs},
		}, nil
	})

	svc, err := NewService(ServiceConfig{
		Authorizer: authorizer,
	})
	require.NoError(t, err)
	return svc
}

func TestNewService_RequiresAuthorizer(t *testing.T) {
	t.Parallel()
	_, err := NewService(ServiceConfig{})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
}

func TestAuthorize_Denied(t *testing.T) {
	t.Parallel()
	svc := newTestService(t, nil) // no allowed verbs
	err := svc.authorize(context.Background())
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func TestAuthorize_Allowed(t *testing.T) {
	t.Parallel()
	svc := newTestService(t, []string{types.VerbCreate})
	err := svc.authorize(context.Background())
	require.NoError(t, err)
}
