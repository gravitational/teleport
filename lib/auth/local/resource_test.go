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
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type ResourceSuite struct {
	bk backend.Backend
}

var _ = check.Suite(&ResourceSuite{})

func (r *ResourceSuite) SetUpTest(c *check.C) {
	var err error

	clock := clockwork.NewFakeClockAt(time.Now())

	r.bk, err = lite.NewWithConfig(context.TODO(), lite.Config{
		Path:             c.MkDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            clock,
	})
	c.Assert(err, check.IsNil)
}

func (r *ResourceSuite) TearDownTest(c *check.C) {
	c.Assert(r.bk.Close(), check.IsNil)
}

func (r *ResourceSuite) dumpResources(c *check.C) []services.Resource {
	startKey := []byte("/")
	endKey := backend.RangeEnd(startKey)
	result, err := r.bk.GetRange(context.TODO(), startKey, endKey, 0)
	c.Assert(err, check.IsNil)
	resources, err := ItemsToResources(result.Items...)
	c.Assert(err, check.IsNil)
	return resources
}

func (r *ResourceSuite) runCreationChecks(c *check.C, resources ...services.Resource) {
	for _, rsc := range resources {
		switch r := rsc.(type) {
		case services.User:
			c.Logf("Creating User: %+v", r)
		default:
		}
	}
	err := CreateResources(context.TODO(), r.bk, resources...)
	c.Assert(err, check.IsNil)
	dump := r.dumpResources(c)
Outer:
	for _, exp := range resources {
		for _, got := range dump {
			if got.GetKind() == exp.GetKind() && got.GetName() == exp.GetName() && got.Expiry() == exp.Expiry() {
				continue Outer
			}
		}
		c.Errorf("Missing expected resource kind=%s,name=%s,expiry=%v", exp.GetKind(), exp.GetName(), exp.Expiry().String())
	}
}

func (r *ResourceSuite) TestUserResource(c *check.C) {
	r.runUserResourceTest(c, false)
}

func (r *ResourceSuite) TestUserResourceWithSecrets(c *check.C) {
	r.runUserResourceTest(c, true)
}

func (r *ResourceSuite) runUserResourceTest(c *check.C, withSecrets bool) {
	alice := newUserTestCase(c, "alice", nil, withSecrets)
	bob := newUserTestCase(c, "bob", nil, withSecrets)
	// Check basic dynamic item creation
	r.runCreationChecks(c, alice, bob)
	// Check that dynamically created item is compatible with service
	s := NewIdentityService(r.bk)
	b, err := s.GetUser("bob", withSecrets)
	c.Assert(err, check.IsNil)
	c.Assert(auth.UsersEquals(bob, b), check.Equals, true, check.Commentf("dynamically inserted user does not match"))
	allUsers, err := s.GetUsers(withSecrets)
	c.Assert(err, check.IsNil)
	c.Assert(len(allUsers), check.Equals, 2, check.Commentf("expected exactly two users"))
	for _, user := range allUsers {
		switch user.GetName() {
		case "alice":
			c.Assert(auth.UsersEquals(alice, user), check.Equals, true, check.Commentf("alice does not match"))
		case "bob":
			c.Assert(auth.UsersEquals(bob, user), check.Equals, true, check.Commentf("bob does not match"))
		default:
			c.Errorf("Unexpected user %q", user.GetName())
		}
	}
}

func (r *ResourceSuite) TestCertAuthorityResource(c *check.C) {
	userCA := test.NewCA(services.UserCA, "example.com")
	hostCA := test.NewCA(services.HostCA, "example.com")
	// Check basic dynamic item creation
	r.runCreationChecks(c, userCA, hostCA)
	// Check that dynamically created item is compatible with service
	s := NewCAService(r.bk)
	err := s.CompareAndSwapCertAuthority(userCA, userCA)
	c.Assert(err, check.IsNil)
}

func (r *ResourceSuite) TestTrustedClusterResource(c *check.C) {
	ctx := context.Background()
	foo, err := services.NewTrustedCluster("foo", services.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{"bar", "baz"},
		Token:                "qux",
		ProxyAddress:         "quux",
		ReverseTunnelAddress: "quuz",
	})
	c.Assert(err, check.IsNil)

	bar, err := services.NewTrustedCluster("bar", services.TrustedClusterSpecV2{
		Enabled:              false,
		Roles:                []string{"baz", "aux"},
		Token:                "quux",
		ProxyAddress:         "quuz",
		ReverseTunnelAddress: "corge",
	})
	c.Assert(err, check.IsNil)
	// Check basic dynamic item creation
	r.runCreationChecks(c, foo, bar)

	s := NewPresenceService(r.bk)
	_, err = s.GetTrustedCluster(ctx, "foo")
	c.Assert(err, check.IsNil)
	_, err = s.GetTrustedCluster(ctx, "bar")
	c.Assert(err, check.IsNil)
}

func (r *ResourceSuite) TestGithubConnectorResource(c *check.C) {
	ctx := context.Background()
	connector := &services.GithubConnectorV3{
		Kind:    services.KindGithubConnector,
		Version: services.V3,
		Metadata: services.Metadata{
			Name:      "github",
			Namespace: defaults.Namespace,
		},
		Spec: services.GithubConnectorSpecV3{
			ClientID:     "aaa",
			ClientSecret: "bbb",
			RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
			Display:      "Github",
			TeamsToLogins: []services.TeamMapping{
				{
					Organization: "gravitational",
					Team:         "admins",
					Logins:       []string{"admin"},
					KubeGroups:   []string{"system:masters"},
				},
			},
		},
	}
	// Check basic dynamic item creation
	r.runCreationChecks(c, connector)

	s := NewIdentityService(r.bk)
	_, err := s.GetGithubConnector(ctx, "github", true)
	c.Assert(err, check.IsNil)
}

func u2fRegTestCase(c *check.C) u2f.Registration {
	derKey, err := base64.StdEncoding.DecodeString("MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEGOi54Eun0r3Xrj8PjyOGYzJObENYI/t/Lr9g9PsHTHnp1qI2ysIhsdMPd7x/vpsL6cr+2EPVik7921OSsVjEMw==")
	c.Assert(err, check.IsNil)
	pubkeyInterface, err := x509.ParsePKIXPublicKey(derKey)
	c.Assert(err, check.IsNil)

	pubkey, ok := pubkeyInterface.(*ecdsa.PublicKey)
	c.Assert(ok, check.Equals, true)

	registration := u2f.Registration{
		Raw:       []byte("BQQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzQIJ+PVFsYjEa0Xgnx+siQaxdlku+U+J2W55U5NrN1iGIc0Amh+0HwhbV2W90G79cxIYS2SVIFAdqTTDXvPXJbeAwggE8MIHkoAMCAQICChWIR0AwlYJZQHcwCgYIKoZIzj0EAwIwFzEVMBMGA1UEAxMMRlQgRklETyAwMTAwMB4XDTE0MDgxNDE4MjkzMloXDTI0MDgxNDE4MjkzMlowMTEvMC0GA1UEAxMmUGlsb3RHbnViYnktMC40LjEtMTU4ODQ3NDAzMDk1ODI1OTQwNzcwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzMAoGCCqGSM49BAMCA0cAMEQCIIbmYKu6I2L4pgZCBms9NIo9yo5EO9f2irp0ahvLlZudAiC8RN/N+WHAFdq8Z+CBBOMsRBFDDJy3l5EDR83B5GAfrjBEAiBl6R6gAmlbudVpW2jSn3gfjmA8EcWq0JsGZX9oFM/RJwIgb9b01avBY5jBeVIqw5KzClLzbRDMY4K+Ds6uprHyA1Y="),
		KeyHandle: []byte("gn49UWxiMRrReCfH6yJBrF2WS75T4nZbnlTk2s3WIYhzQCaH7QfCFtXZb3Qbv1zEhhLZJUgUB2pNMNe89clt4A=="),
		PubKey:    *pubkey,
	}
	return registration
}

func localAuthSecretsTestCase(c *check.C) services.LocalAuthSecrets {
	var secrets services.LocalAuthSecrets
	var err error
	secrets.PasswordHash, err = bcrypt.GenerateFromPassword([]byte("insecure"), bcrypt.MinCost)
	c.Assert(err, check.IsNil)

	dev, err := auth.NewTOTPDevice("otp", base32.StdEncoding.EncodeToString([]byte("abc123")), time.Now())
	c.Assert(err, check.IsNil)
	secrets.MFA = append(secrets.MFA, dev)

	reg := u2fRegTestCase(c)
	dev, err = u2f.NewDevice("u2f", &reg, time.Now())
	c.Assert(err, check.IsNil)
	dev.GetU2F().Counter = 7
	secrets.MFA = append(secrets.MFA, dev)
	return secrets
}

func newUserTestCase(c *check.C, name string, roles []string, withSecrets bool) services.User {
	user := services.UserV2{
		Kind:    services.KindUser,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: services.UserSpecV2{
			Roles: roles,
		},
	}
	if withSecrets {
		auth := localAuthSecretsTestCase(c)
		user.SetLocalAuth(&auth)
	}
	return &user
}
