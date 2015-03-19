// package membk implements in-memory backend used for tests purposes
package membk

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/backend"
)

type MemBackend struct {
	HostCA *backend.CA
	UserCA *backend.CA

	Users   map[string]*User
	Servers map[string]backend.Server
	WebTuns map[string]backend.WebTun
}

type User struct {
	Keys     map[string]backend.AuthorizedKey
	Sessions map[string]backend.WebSession
	Hash     []byte
}

func New() *MemBackend {
	return &MemBackend{
		Users:   make(map[string]*User),
		Servers: make(map[string]backend.Server),
		WebTuns: make(map[string]backend.WebTun),
	}
}

func (b *MemBackend) Close() error {
	return nil
}

// GetUsers  returns a list of users registered in the backend
func (b *MemBackend) GetUsers() ([]string, error) {
	out := []string{}
	for k := range b.Users {
		out = append(out, k)
	}
	return out, nil
}

// DeleteUser deletes a user with all the keys from the backend
func (b *MemBackend) DeleteUser(user string) error {
	_, ok := b.Users[user]
	if !ok {
		return &backend.NotFoundError{}
	}
	delete(b.Users, user)
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
	u, ok := b.Users[user]
	if !ok || len(u.Keys) == 0 {
		return values, nil
	}
	for _, k := range u.Keys {
		values = append(values, k)
	}
	return values, nil
}

func (b *MemBackend) getUser(user string) *User {
	u, ok := b.Users[user]
	if ok {
		return u
	}
	u = &User{
		Keys:     make(map[string]backend.AuthorizedKey),
		Sessions: make(map[string]backend.WebSession),
	}
	b.Users[user] = u
	return u
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
	b.getUser(user).Keys[key.ID] = key
	return nil
}

func (b *MemBackend) DeleteUserKey(user, keyID string) error {
	if user == "" {
		return &backend.MissingParameterError{Param: "user"}
	}
	if keyID == "" {
		return &backend.MissingParameterError{Param: "key.id"}
	}
	u, ok := b.Users[user]
	if !ok {
		return &backend.NotFoundError{}
	}
	if _, ok := u.Keys[keyID]; !ok {
		return &backend.NotFoundError{}
	}
	delete(u.Keys, keyID)
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

func (b *MemBackend) UpsertPasswordHash(user string, hash []byte) error {
	b.getUser(user).Hash = hash
	return nil
}

func (b *MemBackend) GetPasswordHash(user string) ([]byte, error) {
	u, ok := b.Users[user]
	if !ok {
		return nil, &backend.NotFoundError{Message: fmt.Sprintf("user '%v' not found", user)}
	}
	return u.Hash, nil
}

func (b *MemBackend) UpsertWebSession(user, sid string, s backend.WebSession, ttl time.Duration) error {
	b.getUser(user).Sessions[sid] = s
	return nil
}

func (b *MemBackend) GetWebSession(user, sid string) (*backend.WebSession, error) {
	u, ok := b.Users[user]
	if !ok {
		return nil, &backend.NotFoundError{Message: fmt.Sprintf("user '%v' not found", user)}
	}
	ws, ok := u.Sessions[sid]
	if !ok {
		return nil, &backend.NotFoundError{Message: fmt.Sprintf("session '%v' not found for user '%v'", sid, user)}
	}
	return &ws, nil
}

func (b *MemBackend) GetWebSessions(user string) ([]backend.WebSession, error) {
	u, ok := b.Users[user]
	if !ok {
		return nil, &backend.NotFoundError{Message: fmt.Sprintf("user '%v' not found", user)}
	}
	out := []backend.WebSession{}
	if len(u.Sessions) == 0 {
		return out, nil
	}
	for _, ws := range u.Sessions {
		out = append(out, ws)
	}
	return out, nil
}

func (b *MemBackend) DeleteWebSession(user, sid string) error {
	u, ok := b.Users[user]
	if !ok {
		return &backend.NotFoundError{Message: fmt.Sprintf("user '%v' not found", user)}
	}
	if _, ok := u.Sessions[sid]; !ok {
		return &backend.NotFoundError{Message: fmt.Sprintf("session '%v' not found for user '%v'", user, sid)}
	}
	delete(u.Sessions, sid)
	return nil
}

func (b *MemBackend) UpsertWebTun(t backend.WebTun, ttl time.Duration) error {
	if t.Prefix == "" {
		return &backend.MissingParameterError{Param: "Prefix"}
	}
	b.WebTuns[t.Prefix] = t
	return nil
}

func (b *MemBackend) GetWebTun(prefix string) (*backend.WebTun, error) {
	t, ok := b.WebTuns[prefix]
	if !ok {
		return nil, &backend.NotFoundError{Message: fmt.Sprintf("web tunnel '%v' not found", prefix)}
	}
	return &t, nil
}

func (b *MemBackend) DeleteWebTun(prefix string) error {
	_, ok := b.WebTuns[prefix]
	if !ok {
		return &backend.NotFoundError{Message: fmt.Sprintf("web tunnel '%v' not found", prefix)}
	}
	delete(b.WebTuns, prefix)
	return nil
}

func (b *MemBackend) GetWebTuns() ([]backend.WebTun, error) {
	out := []backend.WebTun{}
	for _, t := range b.WebTuns {
		out = append(out, t)
	}
	return out, nil
}
