/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package loginrulev1

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/lib/services"
)

// NotImplementedService is a [loginrulepb.LoginRuleServiceServer] which
// returns errors for all RPCs that indicate that enterprise
// is required to use the service. Using a [loginrulepb.UnimplementedLoginRuleServiceServer]
// would result in ambiguous not implemented errors being returned from open source.
type NotImplementedService struct {
	loginrulepb.UnimplementedLoginRuleServiceServer
}

func (NotImplementedService) CreateLoginRule(context.Context, *loginrulepb.CreateLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) UpsertLoginRule(context.Context, *loginrulepb.UpsertLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) GetLoginRule(context.Context, *loginrulepb.GetLoginRuleRequest) (*loginrulepb.LoginRule, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) ListLoginRules(context.Context, *loginrulepb.ListLoginRulesRequest) (*loginrulepb.ListLoginRulesResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) DeleteLoginRule(context.Context, *loginrulepb.DeleteLoginRuleRequest) (*emptypb.Empty, error) {
	return nil, services.ErrRequiresEnterprise
}
func (NotImplementedService) TestLoginRule(context.Context, *loginrulepb.TestLoginRuleRequest) (*loginrulepb.TestLoginRuleResponse, error) {
	return nil, services.ErrRequiresEnterprise
}
