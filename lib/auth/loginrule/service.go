// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loginrule

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
