package server

import "sync"

// interface for user credential provider
// hint: can be extended for more functionality
// =================================IMPORTANT NOTE===============================
// if the password in a third-party credential provider could be updated at runtime, we have to invalidate the caching
// for 'caching_sha2_password' by calling 'func (s *Server)InvalidateCache(string, string)'.
type CredentialProvider interface {
	// check if the user exists
	CheckUsername(username string) (bool, error)
	// get user credential
	GetCredential(username string) (password string, found bool, err error)
}

func NewInMemoryProvider() *InMemoryProvider {
	return &InMemoryProvider{
		userPool: sync.Map{},
	}
}

// implements a in memory credential provider
type InMemoryProvider struct {
	userPool sync.Map // username -> password
}

func (m *InMemoryProvider) CheckUsername(username string) (found bool, err error) {
	_, ok := m.userPool.Load(username)
	return ok, nil
}

func (m *InMemoryProvider) GetCredential(username string) (password string, found bool, err error) {
	v, ok := m.userPool.Load(username)
	if !ok {
		return "", false, nil
	}
	return v.(string), true, nil
}

func (m *InMemoryProvider) AddUser(username, password string) {
	m.userPool.Store(username, password)
}

type Provider InMemoryProvider
