/*
Copyright 2015 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

type UserService struct {
	backend backend.Backend
}

func NewUserService(backend backend.Backend) *UserService {
	return &UserService{backend}
}

// Upsert Public key in OpenSSH authorized Key format
// user is a user name, keyID is a unique IDentifier for the key
// in case if ttl is 0, the key will be upserted permanently, otherwise
// it will expire in ttl seconds
func (s *UserService) UpsertUserKey(user string, key AuthorizedKey,
	ttl time.Duration) error {
	err := s.backend.UpsertVal([]string{"users", user, "keys"},
		key.ID, key.Value, ttl)
	return err
}

// GetUserKeys returns a list of authorized keys for a given user
// in a OpenSSH key authorized_keys format
func (s *UserService) GetUserKeys(user string) ([]AuthorizedKey, error) {
	IDs, err := s.backend.GetKeys([]string{"users", user, "keys"})
	if err != nil {
		return nil, err
	}

	keys := make([]AuthorizedKey, len(IDs))
	for i, id := range IDs {
		value, err := s.backend.GetVal([]string{"users", user, "keys"},
			id)
		if err != nil {
			log.Errorf(err.Error())
			return nil, trace.Wrap(err)
		}
		keys[i].ID = id
		keys[i].Value = value
	}
	return keys, nil
}

// GetUsers  returns a list of users registered in the backend
func (s *UserService) GetUsers() ([]string, error) {
	users, err := s.backend.GetKeys([]string{"users"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return users, nil
}

// DeleteUser deletes a user with all the keys from the backend
func (s *UserService) DeleteUser(user string) error {
	err := s.backend.DeleteBucket([]string{"users"}, user)
	return err
}

// DeleteUserKey deletes user key by given ID
func (s *UserService) DeleteUserKey(user, key string) error {
	err := s.backend.DeleteKey([]string{"users", user, "keys"}, key)
	return err
}

type AuthorizedKey struct {
	ID    string `json:"id"`
	Value []byte `json:"value"`
}
