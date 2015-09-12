package services

import (
	"encoding/json"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/backend/encryptedbk"
)

type BkKeysService struct {
	backend *encryptedbk.ReplicatedBackend
}

func NewBkKeysService(backend *encryptedbk.ReplicatedBackend) *BkKeysService {
	return &BkKeysService{backend}
}

// GetBackendKeys returns IDs of all the backend encrypting keys that
// this server has
func (s *BkKeysService) GetBackendKeys() ([]string, error) {
	keys, err := s.backend.GetAllEncryptingKeys()
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}
	ids := make([]string, len(keys))
	for i, _ := range keys {
		ids[i] = keys[i].ID
	}
	return ids, nil
}

// GetRemoteBackendKeys returns IDs of all the backend encrypting case
// that are actually used on remote backend
func (s *BkKeysService) GetRemoteBackendKeys() ([]string, error) {
	return s.backend.ListRemoteEncryptingKeys()
}

// GenerateBackendKey generates a new backend encrypting key with the
// given id and then backend makes a copy of all the data using the
// generated key for encryption
func (s *BkKeysService) GenerateBackendKey(keyID string) error {
	return s.backend.GenerateEncryptingKey(keyID, true)
}

// DeleteBackendKey deletes the backend encrypting key and all the data
// encrypted with the key
func (s *BkKeysService) DeleteBackendKey(keyID string) error {
	return s.backend.DeleteEncryptingKey(keyID)
}

// AddBackendKey adds the given encrypting key. If backend works not in
// readonly mode, backend makes a copy of the data using the key for
// encryption
func (s *BkKeysService) AddBackendKey(keyJSON string) (id string, e error) {
	var key encryptedbk.Key
	err := json.Unmarshal([]byte(keyJSON), &key)
	if err != nil {
		log.Errorf(err.Error())
		return "", err
	}

	err = s.backend.AddEncryptingKey(key, true)
	if err != nil {
		log.Errorf(err.Error())
		return "", err
	}
	return key.ID, nil
}

// GetBackendKeys returns the backend encrypting key.
func (s *BkKeysService) GetBackendKey(keyID string) (keyJSON string, e error) {
	key, err := s.backend.GetEncryptingKey(keyID)
	if err != nil {
		log.Errorf(err.Error())
		return "", err
	}
	out, err := json.Marshal(key)
	if err != nil {
		log.Errorf(err.Error())
		return "", err
	}
	return string(out), err
}
