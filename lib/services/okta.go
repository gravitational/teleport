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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// MarshalOktaApplication marshals Okta Application resource to JSON.
func MarshalOktaApp(oktaApp types.OktaApplication, opts ...MarshalOption) ([]byte, error) {
	if err := oktaApp.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	switch oktaApp := oktaApp.(type) {
	case *types.OktaApplicationV1:
		return utils.FastMarshal(oktaApp)
	default:
		return nil, trace.BadParameter("unsupported app resource %T", oktaApp)
	}
}

// UnmarshalOktaApp unmarshals Okta Application resource from JSON.
func UnmarshalOktaApp(data []byte, opts ...MarshalOption) (types.OktaApplication, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing app resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var oktaApp types.OktaApplicationV1
		if err := utils.FastUnmarshal(data, &oktaApp); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := oktaApp.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			oktaApp.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			oktaApp.SetExpiry(cfg.Expires)
		}
		return &oktaApp, nil
	}
	return nil, trace.BadParameter("unsupported Okta app resource version %q", h.Version)
}

// MarshalOktaGroup marshals Okta Group resource to JSON.
func MarshalOktaGroup(oktaGroup types.OktaGroup, opts ...MarshalOption) ([]byte, error) {
	if err := oktaGroup.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	switch oktaGroup := oktaGroup.(type) {
	case *types.OktaGroupV1:
		return utils.FastMarshal(oktaGroup)
	default:
		return nil, trace.BadParameter("unsupported Okta group resource %T", oktaGroup)
	}
}

// UnmarshalOktaGroup unmarshals Okta Group resource from JSON.
func UnmarshalOktaGroup(data []byte, opts ...MarshalOption) (types.OktaGroup, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing app resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var oktaGroup types.OktaGroupV1
		if err := utils.FastUnmarshal(data, &oktaGroup); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		if err := oktaGroup.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			oktaGroup.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			oktaGroup.SetExpiry(cfg.Expires)
		}
		return &oktaGroup, nil
	}
	return nil, trace.BadParameter("unsupported Okta app resource version %q", h.Version)
}
