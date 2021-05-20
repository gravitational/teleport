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
	"encoding/json"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// CreateResources attempts to dynamically create the supplied resources.
// This function returns `trace.AlreadyExistsError` if one or more resources
// would be overwritten, and `trace.NotImplementedError` if any resources
// are of an unsupported type (see `ItemsFromResources(...)`).
//
// NOTE: This function is non-atomic and performs no internal synchronization;
// backend must be locked by caller when operating in parallel environment.
func CreateResources(ctx context.Context, b backend.Backend, resources ...types.Resource) error {
	items, err := ItemsFromResources(resources...)
	if err != nil {
		return trace.Wrap(err)
	}
	// ensure all items do not exist before continuing.
	for _, item := range items {
		_, err = b.Get(ctx, item.Key)
		if !trace.IsNotFound(err) {
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.AlreadyExists("resource %q already exists", string(item.Key))
		}
	}
	// create all items.
	for _, item := range items {
		_, err := b.Create(ctx, item)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ItemsFromResources attempts to convert resources into instances of backend.Item.
// NOTE: this is not necessarily a 1-to-1 conversion.
func ItemsFromResources(resources ...types.Resource) ([]backend.Item, error) {
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
	switch r := resource.(type) {
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

// ItemsToResources converts one or more items into one or more resources.
// NOTE: This is not necessarily a 1-to-1 conversion, and order is not preserved.
func ItemsToResources(items ...backend.Item) ([]types.Resource, error) {
	var resources []types.Resource
	// User resources may be split across multiple items, so we must extract them first.
	users, rem, err := collectUserItems(items)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for uname, uitems := range users {
		user, err := userFromUserItems(uname, uitems)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, user)
	}
	for _, item := range rem {
		rsc, err := itemToResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, rsc)
	}
	return resources, nil
}

// itemToResource attempts to decode the supplied `backend.Item` as one
// of the supported resource types.  If the resource's `kind` does not match
// one of the supported resource types, `trace.NotImplementedError` is returned.
func itemToResource(item backend.Item) (types.Resource, error) {
	var u services.UnknownResource
	if err := u.UnmarshalJSON(item.Value); err != nil {
		return nil, trace.Wrap(err)
	}
	var rsc types.Resource
	var err error
	switch kind := u.GetKind(); kind {
	case services.KindUser:
		rsc, err = itemToUser(item)
	case services.KindCertAuthority:
		rsc, err = itemToCertAuthority(item)
	case services.KindTrustedCluster:
		rsc, err = itemToTrustedCluster(item)
	case services.KindGithubConnector:
		rsc, err = itemToGithubConnector(item)
	case services.KindRole:
		rsc, err = itemToRole(item)
	case services.KindOIDCConnector:
		rsc, err = itemToOIDCConnector(item)
	case services.KindSAMLConnector:
		rsc, err = itemToSAMLConnector(item)
	case types.KindMFADevice:
		rsc, err = itemToMFADevice(item)
	case "":
		return nil, trace.BadParameter("item %q is not a resource (missing field 'kind')", string(item.Key))
	default:
		return nil, trace.NotImplemented("cannot dynamically decode resource of kind %q", kind)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsc, nil
}

// itemFromUser attempts to encode the supplied user as an
// instance of `backend.Item` suitable for storage.
func itemFromUser(user types.User) (*backend.Item, error) {
	if err := services.ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalUser(user)
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

// itemToUser attempts to decode the supplied `backend.Item` as
// a user resource.
func itemToUser(item backend.Item) (types.User, error) {
	user, err := services.UnmarshalUser(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
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
	value, err := services.MarshalCertAuthority(ca)
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

// itemToCertAuthority attempts to decode the supplied `backend.Item` as
// a certificate authority resource (NOTE: does not filter secrets).
func itemToCertAuthority(item backend.Item) (types.CertAuthority, error) {
	ca, err := services.UnmarshalCertAuthority(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateCertAuthority(ca); err != nil {
		return nil, trace.Wrap(err)
	}
	return ca, nil
}

// itemFromTrustedCluster attempts to encode the supplied trusted cluster
// as an instance of `backend.Item` suitable for storage.
func itemFromTrustedCluster(tc types.TrustedCluster) (*backend.Item, error) {
	if err := tc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalTrustedCluster(tc)
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

// itemToTrustedCluster attempts to decode the supplied `backend.Item` as
// a trusted cluster resource.
func itemToTrustedCluster(item backend.Item) (types.TrustedCluster, error) {
	tc, err := services.UnmarshalTrustedCluster(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tc, nil
}

// itemFromGithubConnector attempts to encode the supplied github connector
// as an instance of `backend.Item` suitable for storage.
func itemFromGithubConnector(gc types.GithubConnector) (*backend.Item, error) {
	if err := gc.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalGithubConnector(gc)
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

// itemToGithubConnector attempts to decode the supplied `backend.Item` as
// a github connector resource.
func itemToGithubConnector(item backend.Item) (types.GithubConnector, error) {
	// XXX: The `GithubConnectorMarshaler` interface is an outlier in that it
	// does not support marshal options (e.g. `WithResourceID(..)`).  Support should
	// be added unless this is an intentional omission.
	gc, err := services.UnmarshalGithubConnector(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return gc, nil
}

// itemFromRole attempts to encode the supplied role as an
// instance of `backend.Item` suitable for storage.
func itemFromRole(role types.Role) (*backend.Item, error) {
	value, err := services.MarshalRole(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	item := &backend.Item{
		Key:     backend.Key(rolesPrefix, role.GetName(), paramsPrefix),
		Value:   value,
		Expires: role.Expiry(),
		ID:      role.GetResourceID(),
	}
	return item, nil
}

// itemToRole attempts to decode the supplied `backend.Item` as
// a role resource.
func itemToRole(item backend.Item) (types.Role, error) {
	role, err := services.UnmarshalRole(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// itemFromOIDCConnector attempts to encode the supplied connector as an
// instance of `backend.Item` suitable for storage.
func itemFromOIDCConnector(connector types.OIDCConnector) (*backend.Item, error) {
	if err := connector.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, oidcPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
		ID:      connector.GetResourceID(),
	}
	return item, nil
}

// itemToOIDCConnector attempts to decode the supplied `backend.Item` as
// an oidc connector resource.
func itemToOIDCConnector(item backend.Item) (types.OIDCConnector, error) {
	connector, err := services.UnmarshalOIDCConnector(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connector, nil
}

// itemFromSAMLConnector attempts to encode the supplied connector as an
// instance of `backend.Item` suitable for storage.
func itemFromSAMLConnector(connector types.SAMLConnector) (*backend.Item, error) {
	if err := services.ValidateSAMLConnector(connector); err != nil {
		return nil, trace.Wrap(err)
	}
	value, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := &backend.Item{
		Key:     backend.Key(webPrefix, connectorsPrefix, samlPrefix, connectorsPrefix, connector.GetName()),
		Value:   value,
		Expires: connector.Expiry(),
		ID:      connector.GetResourceID(),
	}
	return item, nil
}

// itemToSAMLConnector attempts to decode the supplied `backend.Item` as
// a saml connector resource.
func itemToSAMLConnector(item backend.Item) (types.SAMLConnector, error) {
	connector, err := services.UnmarshalSAMLConnector(
		item.Value,
		services.WithResourceID(item.ID),
		services.WithExpires(item.Expires),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return connector, nil
}

func itemToMFADevice(item backend.Item) (*types.MFADevice, error) {
	var d types.MFADevice
	err := json.Unmarshal(item.Value, &d)
	return &d, trace.Wrap(err)
}

// userFromUserItems is an extended variant of itemToUser which can be used
// with the `userItems` collector to include additional backend.Item values
// such as password hash or u2f registration.
func userFromUserItems(name string, items userItems) (types.User, error) {
	if items.params == nil {
		return nil, trace.BadParameter("cannot itemTo user %q without primary item %q", name, paramsPrefix)
	}
	user, err := itemToUser(*items.params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if items.Len() < 2 {
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

	// DELETE IN 7.0: these items are migrated to items.mfa on 6.0 first
	// startup.
	//
	// Delete starts here...
	if items.totp != nil {
		auth.TOTPKey = string(items.totp.Value)
	}
	if items.u2fRegistration != nil {
		var raw struct {
			Raw              []byte `json:"raw"`
			KeyHandle        []byte `json:"keyhandle"`
			MarshalledPubKey []byte `json:"marshalled_pubkey"`
		}
		if err := json.Unmarshal(items.u2fRegistration.Value, &raw); err != nil {
			return nil, trace.Wrap(err)
		}
		auth.U2FRegistration = &types.U2FRegistrationData{
			Raw:       raw.Raw,
			KeyHandle: raw.KeyHandle,
			PubKey:    raw.MarshalledPubKey,
		}
	}
	if items.u2fCounter != nil {
		var raw struct {
			Counter uint32 `json:"counter"`
		}
		if err := json.Unmarshal(items.u2fCounter.Value, &raw); err != nil {
			return nil, trace.Wrap(err)
		}
		auth.U2FCounter = raw.Counter
	}
	// ... delete ends here.
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
			Key:   backend.Key(webPrefix, usersPrefix, user, pwdPrefix),
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
			Key:   backend.Key(webPrefix, usersPrefix, user, mfaDevicePrefix, mfa.Id),
			Value: value,
		})
	}
	return items, nil
}

// TODO: convert username/suffix ops to work on bytes by default; string/byte conversion
// has order N cost.

// fullUsersPrefix is the entire string preceding the name of a user in a key
var fullUsersPrefix string = string(backend.Key(webPrefix, usersPrefix)) + "/"

// splitUsernameAndSuffix is a helper for extracting usernames and suffixes from
// backend key values.
func splitUsernameAndSuffix(key string) (name string, suffix string, err error) {
	if !strings.HasPrefix(key, fullUsersPrefix) {
		return "", "", trace.BadParameter("expected format '%s/<name>/<suffix>', got '%s'", fullUsersPrefix, key)
	}
	key = strings.TrimPrefix(key, fullUsersPrefix)
	idx := strings.Index(key, "/")
	if idx < 1 || idx >= len(key) {
		return "", "", trace.BadParameter("expected format <name>/<suffix>, got %q", key)
	}
	return key[:idx], key[idx+1:], nil
}

// collectUserItems handles the case where multiple items pertain to the same user resource.
// User associated items are sorted by username and suffix.  Items which do not both start with
// the expected prefix *and* end with one of the expected suffixes are passed back in `rem`.
func collectUserItems(items []backend.Item) (users map[string]userItems, rem []backend.Item, err error) {
	users = make(map[string]userItems)
	for _, item := range items {
		key := string(item.Key)
		if !strings.HasPrefix(key, fullUsersPrefix) {
			rem = append(rem, item)
			continue
		}
		name, suffix, err := splitUsernameAndSuffix(key)
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
	params *backend.Item
	pwd    *backend.Item
	mfa    []*backend.Item

	// Deprecated fields, only used for migration on auth server startup.
	totp            *backend.Item
	u2fRegistration *backend.Item
	u2fCounter      *backend.Item
}

// Set attempts to set a field by suffix.
func (u *userItems) Set(suffix string, item backend.Item) (ok bool) {
	switch suffix {
	case paramsPrefix:
		u.params = &item
	case pwdPrefix:
		u.pwd = &item

	// DELETE IN 7.0: these items are migrated to mfaDevicePrefix on 6.0 first
	// startup.
	//
	// Delete starts here...
	case totpPrefix:
		u.totp = &item
	case u2fRegistrationPrefix:
		u.u2fRegistration = &item
	case u2fRegistrationCounterPrefix:
		u.u2fCounter = &item
	// ... delete ends here.

	default:
		if strings.HasPrefix(suffix, mfaDevicePrefix) {
			u.mfa = append(u.mfa, &item)
		} else {
			return false
		}
	}
	return true
}

func (u *userItems) Len() int {
	var l int
	if u.params != nil {
		l++
	}
	if u.pwd != nil {
		l++
	}
	l += len(u.mfa)
	return l
}

// complete checks whether userItems is complete enough to be parsed by
// userFromUserItems.
func (u *userItems) complete() bool {
	return u.params != nil
}
