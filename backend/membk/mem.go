// package membk implements in-memory backend used for tests purposes
package membk

import (
	"time"

	"github.com/gravitational/teleport/backend"
)

type MemBackend struct {
	HostCA *backend.CA
	UserCA *backend.CA

	Keys    map[string]map[string]backend.AuthorizedKey
	Servers map[string]backend.Server
}

func New() *MemBackend {
	return &MemBackend{
		Keys:    make(map[string]map[string]backend.AuthorizedKey),
		Servers: make(map[string]backend.Server),
	}
}

func (b *MemBackend) Close() error {
	return nil
}

// GetUsers  returns a list of users registered in the backend
func (b *MemBackend) GetUsers() ([]string, error) {
	out := []string{}
	for k := range b.Keys {
		out = append(out, k)
	}
	return out, nil
}

// DeleteUser deletes a user with all the keys from the backend
func (b *MemBackend) DeleteUser(user string) error {
	_, ok := b.Keys[user]
	if !ok {
		return &backend.NotFoundError{}
	}
	delete(b.Keys, user)
	return nil
}

func (b *MemBackend) UpsertUserCA(a backend.CA) error {
	b.UserCA = &a
	return nil
}

func (b *MemBackend) GetUserCA() (*backend.CA, error) {
	if b.UserCA == nil {
		return nil, &backend.NotFoundError{}
	}
	return b.UserCA, nil
}

func (b *MemBackend) GetUserCAPub() ([]byte, error) {
	ca, err := b.GetUserCA()
	if err != nil {
		return nil, err
	}
	return ca.Pub, nil
}

func (b *MemBackend) UpsertHostCA(a backend.CA) error {
	b.HostCA = &a
	return nil
}

func (b *MemBackend) GetHostCA() (*backend.CA, error) {
	if b.HostCA == nil {
		return nil, &backend.NotFoundError{}
	}
	return b.HostCA, nil
}

func (b *MemBackend) GetHostCAPub() ([]byte, error) {
	ca, err := b.GetHostCA()
	if err != nil {
		return nil, err
	}
	return ca.Pub, nil
}

func (b *MemBackend) GetUserKeys(user string) ([]backend.AuthorizedKey, error) {
	if user == "" {
		return nil, &backend.MissingParameterError{Param: "user"}
	}
	values := []backend.AuthorizedKey{}
	out, ok := b.Keys[user]
	if !ok || len(out) == 0 {
		return values, nil
	}
	for _, k := range out {
		values = append(values, k)
	}
	return values, nil
}

func (b *MemBackend) UpsertUserKey(user string, key backend.AuthorizedKey, ttl time.Duration) error {
	if user == "" {
		return &backend.MissingParameterError{Param: "user"}
	}
	if key.ID == "" {
		return &backend.MissingParameterError{Param: "key.id"}
	}
	if len(key.Value) == 0 {
		return &backend.MissingParameterError{Param: "key.val"}
	}
	if _, ok := b.Keys[user]; !ok {
		b.Keys[user] = map[string]backend.AuthorizedKey{}
	}
	b.Keys[user][key.ID] = key
	return nil
}

func (b *MemBackend) DeleteUserKey(user, keyID string) error {
	if user == "" {
		return &backend.MissingParameterError{Param: "user"}
	}
	if keyID == "" {
		return &backend.MissingParameterError{Param: "key.id"}
	}
	out, ok := b.Keys[user]
	if !ok {
		return &backend.NotFoundError{}
	}
	if _, ok := out[keyID]; !ok {
		return &backend.NotFoundError{}
	}
	delete(out, keyID)
	return nil
}

func (b *MemBackend) UpsertServer(s backend.Server, ttl time.Duration) error {
	b.Servers[s.ID] = s
	return nil
}

func (b *MemBackend) GetServers() ([]backend.Server, error) {
	values := []backend.Server{}
	for _, s := range b.Servers {
		values = append(values, s)
	}
	return values, nil
}
