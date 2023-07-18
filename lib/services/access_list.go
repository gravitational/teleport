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

	accesslistclient "github.com/gravitational/teleport/api/client/accesslist"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/utils"
)

var _ AccessLists = (*accesslistclient.Client)(nil)

// AccessListsGetter defines an interface for reading access lists.
type AccessListsGetter interface {
	// GetAccessLists returns a list of all access lists.
	GetAccessLists(context.Context) ([]*accesslist.AccessList, error)
	// GetAccessList returns the specified access list resource.
	GetAccessList(context.Context, string) (*accesslist.AccessList, error)
}

// AccessLists defines an interface for managing AccessLists.
type AccessLists interface {
	AccessListsGetter

	// UpsertAccessList creates or updates an access list resource.
	UpsertAccessList(context.Context, *accesslist.AccessList) (*accesslist.AccessList, error)
	// DeleteAccessList removes the specified access list resource.
	DeleteAccessList(context.Context, string) error
	// DeleteAllAccessLists removes all access lists.
	DeleteAllAccessLists(context.Context) error
}

// MarshalAccessList marshals the access list resource to JSON.
func MarshalAccessList(accessList *accesslist.AccessList, opts ...MarshalOption) ([]byte, error) {
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *accessList
		copy.SetResourceID(0)
		accessList = &copy
	}
	return utils.FastMarshal(accessList)
}

// UnmarshalAccessList unmarshals the access list resource from JSON.
func UnmarshalAccessList(data []byte, opts ...MarshalOption) (*accesslist.AccessList, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var accessList *accesslist.AccessList
	if err := utils.FastUnmarshal(data, &accessList); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := accessList.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		accessList.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		accessList.SetExpiry(cfg.Expires)
	}
	return accessList, nil
}
