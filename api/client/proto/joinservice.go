/*
Copyright 2022 Gravitational, Inc.

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

package proto

import (
	"github.com/gravitational/trace"
)

func (r *RegisterUsingIAMMethodRequest) CheckAndSetDefaults() error {
	if len(r.StsIdentityRequest) == 0 {
		return trace.BadParameter("missing parameter StsIdentityRequest")
	}
	return trace.Wrap(r.RegisterUsingTokenRequest.CheckAndSetDefaults())
}

func (r *RegisterUsingAzureMethodRequest) CheckAndSetDefaults() error {
	if len(r.AttestedData) == 0 {
		return trace.BadParameter("missing parameter AttestedData")
	}
	if len(r.AccessToken) == 0 {
		return trace.BadParameter("missing parameter AccessToken")
	}
	return trace.Wrap(r.RegisterUsingTokenRequest.CheckAndSetDefaults())
}

func (r *RegisterUsingOracleMethodRequest) CheckAndSetDefaults() error {
	switch req := r.Request.(type) {
	case *RegisterUsingOracleMethodRequest_RegisterUsingTokenRequest:
		return trace.Wrap(req.RegisterUsingTokenRequest.CheckAndSetDefaults())
	case *RegisterUsingOracleMethodRequest_OracleRequest:
		if len(req.OracleRequest.Headers) == 0 {
			return trace.BadParameter("missing parameter Headers")
		}
		if len(req.OracleRequest.PayloadHeaders) == 0 {
			return trace.BadParameter("missing parameter PayloadHeaders")
		}
	}
	return trace.BadParameter("invalid request type: %T", r.Request)
}
