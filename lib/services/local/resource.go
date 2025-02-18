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

package local

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

// CreateResources attempts to dynamically create the supplied resources.
// If any resources already exist they are skipped and not overwritten.
// This function returns a `trace.NotImplementedError` if any resources
// are of an unsupported type (see `itemsFromResources(...)`).
//
// NOTE: This function is non-atomic and performs no internal synchronization;
// backend must be locked by caller when operating in parallel environment.
func CreateResources(ctx context.Context, b backend.Backend, resources ...types.Resource) error {
	items, err := itemsFromResources(resources...)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, item := range items {
		_, err := b.Create(ctx, item)
		if !trace.IsAlreadyExists(err) && err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// itemsFromResources attempts to convert resources into instances of backend.Item.
// NOTE: this is not necessarily a 1-to-1 conversion.
func itemsFromResources(resources ...types.Resource) ([]backend.Item, error) {
	var allItems []backend.Item
	for _, rsc := range resources {
		items, err := itemsFromResource(rsc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allItems = append(allItems, items...)
	}
	return allItems, nil
}

// ItemsFromResource attempts to construct one or more instances of `backend.Item` from
// a given resource.  If `rsc` is not one of the supported resource types,
// a `trace.NotImplementedError` is returned.
func itemsFromResource(resource types.Resource) ([]backend.Item, error) {
	var item *backend.Item
	var extItems []backend.Item
	var err error

	// Unwrap "new style" resources.
	// We always want to switch over the underlying type.
	var res any = resource
	if w, ok := res.(types.Resource153Unwrapper); ok {
		res = w.Unwrap()
	}

	switch r := res.(type) {
	case types.User:
		item, err = itemFromUser(r)
		if auth := r.GetLocalAuth(); err == nil && auth != nil {
			extItems, err = itemsFromLocalAuthSecrets(r.GetName(), *auth)
		}
	case types.CertAuthority:
		item, err = itemFromCertAuthority(r)
	case types.TrustedCluster:
		item, err = itemFromTrustedCluster(r)
	case types.GithubConnector:
		item, err = itemFromGithubConnector(r)
	case types.Role:
		item, err = itemFromRole(r)
	case types.OIDCConnector:
		item, err = itemFromOIDCConnector(r)
	case types.SAMLConnector:
		item, err = itemFromSAMLConnector(r)
	case types.ProvisionToken:
		item, err = itemFromProvisionToken(r)
	case types.Lock:
		item, err = itemFromLock(r)
	case types.ClusterNetworkingConfig:
		item, err = itemFromClusterNetworkingConfig(r)
	case types.AuthPreference:
		item, err = itemFromAuthPreference(r)
	case *autoupdatev1pb.AutoUpdateConfig:
		item, err = itemFromAutoUpdateConfig(r)
	case *autoupdatev1pb.AutoUpdateVersion:
		item, err = itemFromAutoUpdateVersion(r)
	case types.Server:
		switch r.GetKind() {
		case types.KindNode:
			item, err = itemFromNode(r)
		default:
			return nil, trace.NotImplemented("connot itemFrom unsupported server kind %q", r.GetKind())
		}
	default:
		return nil, trace.NotImplemented("cannot itemFrom resource of type %T", resource)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items := make([]backend.Item, 0, len(extItems)+1)
	items = append(items, *item)
	items = append(items, extItems...)
	return items, nil
}

// itemFromClusterNetworkingConfig attempts to encode the supplied cluster_networking_config as an
// instance of `backend.Item` suitable for storage.
func itemFromClusterNetworkingConfig(cnc types.ClusterNetworkingConfig) (*backend.Item, error) {
	if err := services.CheckAndSetDefaults(cnc); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalClusterNetworkingConfig(cnc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := &backend.Item{
		Key:      backend.NewKey(clusterConfigPrefix, networkingPrefix),
		Value:    value,
		Revision: cnc.GetRevision(),
	}
	return item, nil
}

// itemFromAuthPreference attempts to encode the supplied cluster_auth_preference as an
// instance of `backend.Item` suitable for storage.
func itemFromAuthPreference(ap types.AuthPreference) (*backend.Item, error) {
	if err := services.CheckAndSetDefaults(ap); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalAuthPreference(ap)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := &backend.Item{
		Key:      backend.NewKey(authPrefix, preferencePrefix, generalPrefix),
		Value:    value,
		Revision: ap.GetRevision(),
	}

	return item, nil
}

// itemFromUser attempts to encode the supplied user as an
// instance of `backend.Item` suitable for storage.
func itemFromUser(user types.User) (*backend.Item, error) {
	if err := services.ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := user.GetRevision()
	value, err := services.MarshalUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(webPrefix, usersPrefix, user.GetName(), paramsPrefix),
		Value:    value,
		Expires:  user.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// itemToUser attempts to decode the supplied `backend.Item` as
// a user resource.
func itemToUser(item backend.Item) (*types.UserV2, error) {
	user, err := services.UnmarshalUser(
		item.Value,
		services.WithExpires(item.Expires),
		services.WithRevision(item.Revision),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// itemFromCertAuthority attempts to encode the supplied certificate authority
// as an instance of `backend.Item` suitable for storage.
func itemFromCertAuthority(ca types.CertAuthority) (*backend.Item, error) {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := ca.GetRevision()
	value, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(authoritiesPrefix, string(ca.GetType()), ca.GetName()),
		Value:    value,
		Expires:  ca.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// itemFromProvisionToken attempts to encode the supplied provision token
// as an instance of `backend.Item` suitable for storage.
func itemFromProvisionToken(p types.ProvisionToken) (*backend.Item, error) {
	if err := services.CheckAndSetDefaults(p); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := p.GetRevision()
	value, err := services.MarshalProvisionToken(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(tokensPrefix, p.GetName()),
		Value:    value,
		Expires:  p.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// itemFromTrustedCluster attempts to encode the supplied trusted cluster
// as an instance of `backend.Item` suitable for storage.
func itemFromTrustedCluster(tc types.TrustedCluster) (*backend.Item, error) {
	if err := services.CheckAndSetDefaults(tc); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := tc.GetRevision()
	value, err := services.MarshalTrustedCluster(tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(trustedClustersPrefix, tc.GetName()),
		Value:    value,
		Expires:  tc.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// itemFromGithubConnector attempts to encode the supplied github connector
// as an instance of `backend.Item` suitable for storage.
func itemFromGithubConnector(gc types.GithubConnector) (*backend.Item, error) {
	if err := services.CheckAndSetDefaults(gc); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := gc.GetRevision()
	value, err := services.MarshalGithubConnector(gc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, githubPrefix, connectorsPrefix, gc.GetName()),
		Value:    value,
		Expires:  gc.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// itemFromRole attempts to encode the supplied role as an
// instance of `backend.Item` suitable for storage.
func itemFromRole(role types.Role) (*backend.Item, error) {
	rev := role.GetRevision()
	value, err := services.MarshalRole(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := &backend.Item{
		Key:      backend.NewKey(rolesPrefix, role.GetName(), paramsPrefix),
		Value:    value,
		Expires:  role.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// itemFromOIDCConnector attempts to encode the supplied connector as an
// instance of `backend.Item` suitable for storage.
func itemFromOIDCConnector(connector types.OIDCConnector) (*backend.Item, error) {
	rev := connector.GetRevision()
	value, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// itemFromSAMLConnector attempts to encode the supplied connector as an
// instance of `backend.Item` suitable for storage.
func itemFromSAMLConnector(connector types.SAMLConnector) (*backend.Item, error) {
	rev := connector.GetRevision()
	if err := services.ValidateSAMLConnector(connector, nil); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:      backend.NewKey(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:    value,
		Expires:  connector.Expiry(),
		Revision: rev,
	}
	return item, nil
}

// userFromUserItems is an extended variant of itemToUser which can be used
// with the `userItems` collector to include additional backend.Item values
// such as password hash or MFA devices.
func userFromUserItems(name string, items userItems) (*types.UserV2, error) {
	if items.params == nil {
		return nil, trace.BadParameter("cannot itemTo user %q without primary item %q", name, paramsPrefix)
	}
	user, err := itemToUser(*items.params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !items.hasLocalAuthItems() {
		return user, nil
	}
	auth, err := itemToLocalAuthSecrets(items)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetLocalAuth(auth)

	return user, nil
}

func itemToLocalAuthSecrets(items userItems) (*types.LocalAuthSecrets, error) {
	var auth types.LocalAuthSecrets
	if items.pwd != nil {
		auth.PasswordHash = items.pwd.Value
	}
	for _, mfa := range items.mfa {
		var d types.MFADevice
		if err := json.Unmarshal(mfa.Value, &d); err != nil {
			return nil, trace.Wrap(err)
		}
		auth.MFA = append(auth.MFA, &d)
	}
	if items.webauthnLocalAuth != nil {
		wal := &types.WebauthnLocalAuth{}
		err := json.Unmarshal(items.webauthnLocalAuth.Value, wal)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		auth.Webauthn = wal
	}
	if err := services.ValidateLocalAuthSecrets(&auth); err != nil {
		return nil, trace.Wrap(err)
	}
	return &auth, nil
}

func itemsFromLocalAuthSecrets(user string, auth types.LocalAuthSecrets) ([]backend.Item, error) {
	var items []backend.Item
	if err := services.ValidateLocalAuthSecrets(&auth); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(auth.PasswordHash) > 0 {
		item := backend.Item{
			Key:   backend.NewKey(webPrefix, usersPrefix, user, pwdPrefix),
			Value: auth.PasswordHash,
		}
		items = append(items, item)
	}
	for _, mfa := range auth.MFA {
		value, err := json.Marshal(mfa)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items = append(items, backend.Item{
			Key:   backend.NewKey(webPrefix, usersPrefix, user, mfaDevicePrefix, mfa.Id),
			Value: value,
		})
	}
	return items, nil
}

// itemFromLock attempts to encode the supplied lock as an
// instance of `backend.Item` suitable for storage.
func itemFromLock(l types.Lock) (*backend.Item, error) {
	if err := services.CheckAndSetDefaults(l); err != nil {
		return nil, trace.Wrap(err)
	}
	rev := l.GetRevision()
	value, err := services.MarshalLock(l)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &backend.Item{
		Key:      backend.NewKey(locksPrefix, l.GetName()),
		Value:    value,
		Expires:  l.Expiry(),
		Revision: rev,
	}, nil
}

// TODO: convert username/suffix ops to work on bytes by default; string/byte conversion
// has order N cost.

// fullUsersPrefix is the entire string preceding the name of a user in a key
var fullUsersPrefix = backend.ExactKey(webPrefix, usersPrefix)

// splitUsernameAndSuffix is a helper for extracting usernames and suffixes from
// backend key values.
func splitUsernameAndSuffix(key backend.Key) (name string, suffix []string, err error) {
	if !key.HasPrefix(fullUsersPrefix) {
		return "", nil, trace.BadParameter("expected format '%s/<name>/<suffix>', got '%s'", fullUsersPrefix, key)
	}
	k := key.TrimPrefix(fullUsersPrefix)

	components := k.Components()
	if len(components) < 2 {
		return "", nil, trace.BadParameter("expected format <name>/<suffix>, got %q", key)
	}
	return components[0], k.Components()[1:], nil
}

// collectUserItems handles the case where multiple items pertain to the same user resource.
// User associated items are sorted by username and suffix.  Items which do not both start with
// the expected prefix *and* end with one of the expected suffixes are passed back in `rem`.
func collectUserItems(items []backend.Item) (users map[string]userItems, rem []backend.Item, err error) {
	users = make(map[string]userItems)
	for _, item := range items {
		if !item.Key.HasPrefix(fullUsersPrefix) {
			rem = append(rem, item)
			continue
		}
		name, suffix, err := splitUsernameAndSuffix(item.Key)
		if err != nil {
			return nil, nil, err
		}
		collector := users[name]
		if !collector.Set(suffix, item) {
			// suffix not recognized, output this item with the rest of the
			// unhandled items.
			rem = append(rem, item)
			continue
		}
		users[name] = collector
	}
	// Remove user entries that are incomplete.
	//
	// For example, an SSO user entry that expired but still has MFA devices
	// persisted. These users should not be loaded until they login again.
	for user, items := range users {
		if !items.complete() {
			delete(users, user)
		}
	}
	return users, rem, nil
}

// userItems is a collector for item types related to a single user resource.
type userItems struct {
	params            *backend.Item
	pwd               *backend.Item
	mfa               []*backend.Item
	webauthnLocalAuth *backend.Item
}

// Set attempts to set a field by suffix.
func (u *userItems) Set(suffix []string, item backend.Item) (ok bool) {
	switch {
	case len(suffix) == 0:
		return false
	case suffix[0] == paramsPrefix:
		u.params = &item
	case suffix[0] == pwdPrefix:
		u.pwd = &item
	case suffix[0] == webauthnLocalAuthPrefix:
		u.webauthnLocalAuth = &item
	case suffix[0] == mfaDevicePrefix:
		u.mfa = append(u.mfa, &item)
	default:
		return false
	}
	return true
}

// complete checks whether userItems is complete enough to be parsed by
// userFromUserItems.
func (u *userItems) complete() bool {
	return u.params != nil
}

func (u *userItems) hasLocalAuthItems() bool {
	return u.pwd != nil || u.webauthnLocalAuth != nil || len(u.mfa) > 0
}
