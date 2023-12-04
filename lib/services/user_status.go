/*
Copyright 2023 Gravitational, Inc.

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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/utils"
)

// UserStatusGetter is the interface for reading user login states.
type UserStatusGetter interface {
	GetUserStatus(context.Context, string) (*userloginstate.UserLoginState, error)
}

// UserStatus foobar
type UserStatus interface {
	UserStatusGetter
	UpdateUserStatus(context.Context, *types.UserStatus) (*types.UserStatus, error)
}

// MarshalUserStatus fdsafdas
func MarshalUserStatus(in *types.UserStatus, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(in)
}

// UnmarshalUserStatus unmarshals a security report state.
func UnmarshalUserStatus(data []byte, opts ...MarshalOption) (*types.UserStatus, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing data")
	}
	var out *types.UserStatus
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	return out, nil
}
