/*
Copyright 2019 Gravitational, Inc.

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

// CreateResources attempts to dynamically create the supplied resources.
// This function returns `trace.AlreadyExistsError` if one or more resources
// would be overwritten, and `trace.NotImplementedError` if any resources
// are of an unsupported type (see `ItemizeResource(...)`).
//
// NOTE: This function is non-atomic and performs no internal synchronization;
// backend must be locked by caller when operating in parallel environment.
func CreateResources(b backend.Backend, resources ...services.Resource) error {
	var items []*backend.Item
	// itemize all resources & ensure that they do not exist.
	for _, r := range resources {
		item, err := ItemizeResource(r)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = b.Get(context.TODO(), item.Key)
		if !trace.IsNotFound(err) {
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.AlreadyExists("resource %q already exists", string(item.Key))
		}
		items = append(items, item)
	}
	// create all items.
	for _, item := range items {
		_, err := b.Create(context.TODO(), *item)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// unmarshalResource attempts to decode an unknown resource into one of the commonly
// used concrete resource types.  If the resource's `kind` does not match one of
// the supported resource types, `trace.NotImplementedError` is returned.
func unmarshalResource(u *services.UnknownResource) (services.Resource, error) {
	var rsc services.Resource
	var err error
	switch kind := u.GetKind(); kind {
	case services.KindUser:
		rsc, err = services.GetUserMarshaler().UnmarshalUser(u.Raw)
	case services.KindCertAuthority:
		rsc, err = services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(u.Raw)
	case services.KindTrustedCluster:
		rsc, err = services.GetTrustedClusterMarshaler().Unmarshal(u.Raw)
	case services.KindGithubConnector:
		rsc, err = services.GetGithubConnectorMarshaler().Unmarshal(u.Raw)
	default:
		return nil, trace.NotImplemented("cannot dynamically unmarshal resource of kind %v", kind)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsc, nil
}

// ItemizeResource attempts to construct an instance of `backend.Item` from
// a given resource.  If `rsc` is not one of the supported resource types,
// a `trace.NotImplementedError` is returned.
func ItemizeResource(resource services.Resource) (*backend.Item, error) {
	// If resource is of unknown type, attempt to cast it to a concrete
	// type before continuing.
	if u, ok := resource.(*services.UnknownResource); ok {
		r, err := unmarshalResource(u)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resource = r
	}
	var item *backend.Item
	var err error
	switch r := resource.(type) {
	case services.User:
		item, err = itemizeUser(r)
	case services.CertAuthority:
		item, err = itemizeCertAuthority(r)
	case services.TrustedCluster:
		item, err = itemizeTrustedCluster(r)
	case services.GithubConnector:
		item, err = itemizeGithubConnector(r)
	default:
		return nil, trace.NotImplemented("cannot itemize resource of kind %v", resource.GetKind())
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return item, nil
}

// DeitemizeResource attempts to decode the supplied `backend.Item` as one
// of the supported resource types.  If the resource's `kind` does not match
// one of the supported resource types, `trace.NotImplementedError` is returned.
func DeitemizeResource(item backend.Item) (services.Resource, error) {
	var u services.UnknownResource
	if err := u.UnmarshalJSON(item.Value); err != nil {
		return nil, trace.Wrap(err)
	}
	var rsc services.Resource
	var err error
	switch kind := u.GetKind(); kind {
	case services.KindUser:
		rsc, err = deitemizeUser(item)
	case services.KindCertAuthority:
		rsc, err = deitemizeCertAuthority(item)
	case services.KindTrustedCluster:
		rsc, err = deitemizeTrustedCluster(item)
	case services.KindGithubConnector:
		rsc, err = deitemizeGithubConnector(item)
	default:
		return nil, trace.NotImplemented("cannot dynamically decode resource of kind %v", kind)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsc, nil
}

// itemizeUser attempts to encode the supplied user as an
// instance of `backend.Item` suitable for storage.
func itemizeUser(user services.User) (*backend.Item, error) {
	if err := user.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.GetUserMarshaler().MarshalUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:     backend.Key(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:   value,
		Expires: user.Expiry(),
		ID:      user.GetResourceID(),
	}
	return item, nil
}

// deitemizeUser attempts to decode the supplied `backend.Item` as
// a user resource.
func deitemizeUser(item backend.Item) (services.User, error) {
	user, err := services.GetUserMarshaler().UnmarshalUser(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := user.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// itemizeCertAuthority attempts to encode the supplied certificate authority
// as an instance of `backend.Item` suitable for storage.
func itemizeCertAuthority(ca services.CertAuthority) (*backend.Item, error) {
	if err := ca.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:     backend.Key(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:   value,
		Expires: ca.Expiry(),
		ID:      ca.GetResourceID(),
	}
	return item, nil
}

// deitemizeCertAuthority attempts to decode the supplied `backend.Item` as
// a certificate authority resource (NOTE: does not filter secrets).
func deitemizeCertAuthority(item backend.Item) (services.CertAuthority, error) {
	ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ca.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, nil
}

// itemizeTrustedCluster attempts to encode the supplied trusted cluster
// as an instance of `backend.Item` suitable for storage.
func itemizeTrustedCluster(tc services.TrustedCluster) (*backend.Item, error) {
	if err := tc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.GetTrustedClusterMarshaler().Marshal(tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:     backend.Key(trustedClustersPrefix, tc.GetName()),
		Value:   value,
		Expires: tc.Expiry(),
		ID:      tc.GetResourceID(),
	}
	return item, nil
}

// deitemizeTrustedCluster attempts to decode the supplied `backend.Item` as
// a trusted cluster resource.
func deitemizeTrustedCluster(item backend.Item) (services.TrustedCluster, error) {
	tc, err := services.GetTrustedClusterMarshaler().Unmarshal(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tc, nil
}

// itemizeGithubConnector attempts to encode the supplied github connector
// as an instance of `backend.Item` suitable for storage.
func itemizeGithubConnector(gc services.GithubConnector) (*backend.Item, error) {
	if err := gc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.GetGithubConnectorMarshaler().Marshal(gc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, gc.GetName()),
		Value:   value,
		Expires: gc.Expiry(),
		ID:      gc.GetResourceID(),
	}
	return item, nil
}

// deitemizeGithubConnector attempts to decode the supplied `backend.Item` as
// a github connector resource.
func deitemizeGithubConnector(item backend.Item) (services.GithubConnector, error) {
	// XXX: The `GithubConnectorMarshaler` interface is an outlier in that it
	// does not support marshal options (e.g. `WithResourceID(..)`).  Support should
	// be added unless this is an intentional omission.
	gc, err := services.GetGithubConnectorMarshaler().Unmarshal(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return gc, nil
}
