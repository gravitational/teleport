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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/utils"
)

// MarshalUserLoginState marshals the user login state resource to JSON.
func MarshalUserLoginState(userLoginState *userloginstate.UserLoginState, opts ...MarshalOption) ([]byte, error) {
	if err := userLoginState.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		prevID := userLoginState.GetResourceID()
		defer func() { userLoginState.SetResourceID(prevID) }()
		userLoginState.SetResourceID(0)
	}
	return utils.FastMarshal(userLoginState)
}

// UnmarshalUserLoginState unmarshals the user login state resource from JSON.
func UnmarshalUserLoginState(data []byte, opts ...MarshalOption) (*userloginstate.UserLoginState, error) {
	//if len(data) == 0 {
	//	return nil, trace.BadParameter("missing user login state data")
	//}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uls := &userloginstate.UserLoginState{}
	if err := utils.FastUnmarshal(data, &uls); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := uls.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	uls.SetResourceID(cfg.ID)
	uls.SetExpiry(cfg.Expires)

	return uls, nil
}
