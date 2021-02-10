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

package test

import (
	"context"
	"crypto/rsa"
	"crypto/x509/pkix"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// NewCA returns new test authority with a test key as a public and
// signing key
func NewCA(caType types.CertAuthType, clusterName string, privateKeys ...[]byte) *types.CertAuthorityV2 {
	return NewCAWithConfig(CAConfig{
		Type:        caType,
		ClusterName: clusterName,
		PrivateKeys: privateKeys,
		Clock:       clockwork.NewRealClock(),
	})
}

// CAConfig defines the configuration for generating
// a test certificate authority
type CAConfig struct {
	Type        types.CertAuthType
	ClusterName string
	PrivateKeys [][]byte
	Clock       clockwork.Clock
}

// NewCAWithConfig generates a new certificate authority with the specified
// configuration
func NewCAWithConfig(config CAConfig) *types.CertAuthorityV2 {
	// privateKeys is to specify another RSA private key
	if len(config.PrivateKeys) == 0 {
		config.PrivateKeys = [][]byte{fixtures.PEMBytes["rsa"]}
	}
	keyBytes := config.PrivateKeys[0]
	rsaKey, err := ssh.ParseRawPrivateKey(keyBytes)
	if err != nil {
		panic(err)
	}

	signer, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		panic(err)
	}

	key, cert, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		PrivateKey: rsaKey.(*rsa.PrivateKey),
		Entity: pkix.Name{
			CommonName:   config.ClusterName,
			Organization: []string{config.ClusterName},
		},
		TTL:   defaults.CATTL,
		Clock: config.Clock,
	})
	if err != nil {
		panic(err)
	}

	publicKey, privateKey, err := jwt.GenerateKeyPair()
	if err != nil {
		panic(err)
	}

	return &types.CertAuthorityV2{
		Kind:    types.KindCertAuthority,
		SubKind: string(config.Type),
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      config.ClusterName,
			Namespace: defaults.Namespace,
		},
		Spec: types.CertAuthoritySpecV2{
			Type:         config.Type,
			ClusterName:  config.ClusterName,
			CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(signer.PublicKey())},
			SigningKeys:  [][]byte{keyBytes},
			TLSKeyPairs:  []types.TLSKeyPair{{Cert: cert, Key: key}},
			JWTKeyPairs: []types.JWTKeyPair{
				{
					PublicKey:  publicKey,
					PrivateKey: privateKey,
				},
			},
		},
	}
}

// CreateUserRoleAndRequestable creates two roles for a user, one base role with allowed login
// matching username, and another role with a login matching rolename that can be requested.
func CreateUserRoleAndRequestable(clt clt, username string, rolename string) (services.User, error) {
	ctx := context.TODO()
	user, err := services.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	baseRole := auth.RoleForUser(user)
	baseRole.SetAccessRequestConditions(services.Allow, services.AccessRequestConditions{
		Roles: []string{rolename},
	})
	baseRole.SetLogins(services.Allow, nil)
	err = clt.UpsertRole(ctx, baseRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(baseRole.GetName())
	err = clt.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	requestableRole := auth.RoleForUser(user)
	requestableRole.SetName(rolename)
	requestableRole.SetLogins(services.Allow, []string{rolename})
	err = clt.UpsertRole(ctx, requestableRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// CreateAccessPluginUser creates a user with list/read abilites for access requests, and list/read/update
// abilities for access plugin data.
func CreateAccessPluginUser(ctx context.Context, clt clt, username string) (services.User, error) {
	user, err := services.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role := auth.RoleForUser(user)
	rules := role.GetRules(types.Allow)
	rules = append(rules,
		types.Rule{
			Resources: []string{types.KindAccessRequest},
			Verbs:     []string{types.VerbRead, types.VerbList},
		},
		types.Rule{
			Resources: []string{types.KindAccessPluginData},
			Verbs:     []string{types.VerbRead, types.VerbList, types.VerbUpdate},
		},
	)
	role.SetRules(types.Allow, rules)
	role.SetLogins(types.Allow, nil)
	if err := clt.UpsertRole(ctx, role); err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	if err := clt.UpsertUser(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// CreateUser creates user and role and assignes role to a user, used in tests
func CreateUser(clt clt, username string, roles ...types.Role) (types.User, error) {
	ctx := context.TODO()
	user, err := services.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, role := range roles {
		err = clt.UpsertRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		user.AddRole(role.GetName())
	}

	err = clt.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// CreateUserAndRole creates user and role and assignes role to a user, used in tests
func CreateUserAndRole(clt clt, username string, allowedLogins []string) (services.User, services.Role, error) {
	ctx := context.TODO()
	user, err := services.NewUser(username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	role := auth.RoleForUser(user)
	role.SetLogins(services.Allow, []string{user.GetName()})
	err = clt.UpsertRole(ctx, role)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	err = clt.UpsertUser(user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return user, role, nil
}

// CreateUserAndRoleWithoutRoles creates user and role, but does not assign user to a role, used in tests
func CreateUserAndRoleWithoutRoles(clt clt, username string, allowedLogins []string) (services.User, services.Role, error) {
	ctx := context.TODO()
	user, err := services.NewUser(username)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	role := auth.RoleForUser(user)
	set := auth.MakeRuleSet(role.GetRules(services.Allow))
	delete(set, services.KindRole)
	role.SetRules(services.Allow, set.Slice())
	role.SetLogins(services.Allow, []string{user.GetName()})
	err = clt.UpsertRole(ctx, role)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	user.AddRole(role.GetName())
	err = clt.UpsertUser(user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return user, role, nil
}

// ExpectResource expects a Put event of a certain resource
func ExpectResource(c *check.C, w services.Watcher, timeout time.Duration, resource services.Resource) {
	timeoutC := time.After(timeout)
waitLoop:
	for {
		select {
		case <-timeoutC:
			c.Fatalf("Timeout waiting for event")
		case <-w.Done():
			c.Fatalf("Watcher exited with error %v", w.Error())
		case event := <-w.Events():
			if event.Type != backend.OpPut {
				log.Debugf("Skipping event %v %v", event.Type, event.Resource.GetName())
				continue
			}
			if resource.GetResourceID() > event.Resource.GetResourceID() {
				log.Debugf("Skipping stale event %v %v %v %v, latest object version is %v", event.Type, event.Resource.GetKind(), event.Resource.GetName(), event.Resource.GetResourceID(), resource.GetResourceID())
				continue waitLoop
			}
			if resource.GetName() != event.Resource.GetName() || resource.GetKind() != event.Resource.GetKind() || resource.GetSubKind() != event.Resource.GetSubKind() {
				log.Debugf("Skipping event %v resource %v, expecting %v", event.Type, event.Resource.GetMetadata(), event.Resource.GetMetadata())
				continue waitLoop
			}
			fixtures.DeepCompare(c, resource, event.Resource)
			break waitLoop
		}
	}
}

// ExpectDeleteResource expects a delete event of a certain kind
func ExpectDeleteResource(c *check.C, w services.Watcher, timeout time.Duration, resource services.Resource) {
	timeoutC := time.After(timeout)
waitLoop:
	for {
		select {
		case <-timeoutC:
			c.Fatalf("Timeout waiting for delete resource %v", resource)
		case <-w.Done():
			c.Fatalf("Watcher exited with error %v", w.Error())
		case event := <-w.Events():
			if event.Type != backend.OpDelete {
				log.Debugf("Skipping stale event %v %v", event.Type, event.Resource.GetName())
				continue
			}
			fixtures.DeepCompare(c, resource, event.Resource)
			break waitLoop
		}
	}
}

// clt limits required interface to the necessary methods
// used to pass different clients in tests
type clt interface {
	UpsertRole(context.Context, types.Role) error
	UpsertUser(types.User) error
}
