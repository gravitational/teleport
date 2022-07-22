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

package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"github.com/form3tech-oss/jwt-go"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/idemeumjwt"
	"gopkg.in/square/go-jose.v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type IdemeumSuite struct {
	a          *Server
	b          backend.Backend
	c          clockwork.FakeClock
	privateKey *ecdsa.PrivateKey
}

// fakeIdemeum is a configurable Idemeum  IdP that can be used to mock responses in
// tests. At the moment it creates an HTTP server and only responds to the
// "/.well-known/jwks.json" endpoint.
type fakeIdemeumServer struct {
	s             *httptest.Server
	jwtSigningKey *ecdsa.PrivateKey
}

func newFakeIdemeumServer(t *testing.T) *fakeIdemeumServer {
	var s fakeIdemeumServer

	mux := http.NewServeMux()
	//for now host the idemeum jwks.json endpoint only
	mux.HandleFunc("/.well-known/jwks.json", s.jwksHandler)
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s.jwtSigningKey = priv
	s.s = httptest.NewServer(mux)
	t.Cleanup(s.s.Close)
	return &s
}

// jwksHandler jwks handler for jwks endpoint
func (s *fakeIdemeumServer) jwksHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var set jose.JSONWebKeySet

	set.Keys = append(set.Keys,
		jose.JSONWebKey{
			Key: &s.jwtSigningKey.PublicKey})
	if err := json.NewEncoder(w).Encode(set); err != nil {
		panic(err)
	}
}

func setUpIdemeumSuite(t *testing.T) *IdemeumSuite {
	s := IdemeumSuite{}

	ctx := context.Background()
	s.c = clockwork.NewFakeClockAt(time.Now())

	var err error
	s.b, err = memory.New(memory.Config{
		Context: ctx,
		Clock:   s.c,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.b,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewServer(authConfig)
	require.NoError(t, err)

	return &s
}

func TestCreateIdemeumUser(t *testing.T) {
	t.Parallel()

	s := setUpIdemeumSuite(t)

	// Create Idemeum user with 1 minute expiry.
	_, err := s.a.createIdemeumUser(&createUserParams{
		connectorName: constants.Idemeum,
		username:      "foo@example.com",
		roles:         []string{"editor"},
		sessionTTL:    1 * time.Minute,
	})
	require.NoError(t, err)

	// Within that 1 minute period the user should still exist.
	user, err := s.a.GetUser("foo@example.com", false)
	require.NoError(t, err)
	require.Equal(t, "foo@example.com", user.GetName())

	// Advance time 2 minutes, the user should be gone.
	s.c.Advance(2 * time.Minute)
	_, err = s.a.GetUser("foo@example.com", false)
	require.Error(t, err)
}

func TestValidateIdemeumToken(t *testing.T) {
	t.Parallel()
	idemeumServer := newFakeIdemeumServer(t)
	//create a keypair
	//create a signed jwt token
	//set up fake tenant server
	TenantUrl := idemeumServer.s.URL
	log.Printf("Idemeum Server Url: %v\n", TenantUrl)
	ServiceToken, err := createSignedJwt(idemeumServer.jwtSigningKey, TenantUrl)
	require.NoError(t, err)
	actualUserParams, err := validateIdemeumToken(ServiceToken, TenantUrl)
	require.NoError(t, err)
	expectedUserParams := &createUserParams{
		connectorName: constants.Idemeum,
		username:      "system",
		roles:         []string{"editor"},
		sessionTTL:    108000000000000}
	require.Equal(t, expectedUserParams, actualUserParams)
}

func createSignedJwt(privateKey *ecdsa.PrivateKey, Issuer string) (string, error) {
	// Sign a token with the new key.
	notBefore := time.Now().Unix()

	claims := idemeumjwt.IdemeumClaims{
		Issuer:    Issuer,
		NotBefore: notBefore,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		Id:        time.Now().String(),
		Subject:   "system",
		Roles:     []string{"editor"},
		Audience:  Issuer,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	return token.SignedString(privateKey)
}
