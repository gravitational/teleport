package services

import (
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/backend"
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
		log.Errorf(err.Error())
		return nil, convertErr(err)
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
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}
	return users, nil
}

// DeleteUser deletes a user with all the keys from the backend
func (s *UserService) DeleteUser(user string) error {
	err := s.backend.DeleteBucket([]string{"users"}, user)
	if err != nil {
		log.Errorf(err.Error())
	}
	return convertErr(err)
}

// DeleteUserKey deletes user key by given ID
func (s *UserService) DeleteUserKey(user, key string) error {
	err := s.backend.DeleteKey([]string{"users", user, "keys"}, key)
	if err != nil {
		log.Errorf(err.Error())
	}
	return convertErr(err)
}

type AuthorizedKey struct {
	ID    string `json:"id"`
	Value []byte `json:"value"`
}
