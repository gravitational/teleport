/*
Copyright 2022 Gravitational, Inc.
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

package client

import (
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"
)

// ClientStore is a storage interface for client data. ClientStore is made up three types
// of data stores.
//
// A ClientStore can be made up of partial data stores with different backends. For example,
// when using `tsh --add-keys-to-agent=only`, ClientStore will be made up of an in-memory
// key store and an FS (~/.tsh) profile and trusted certs store.
type ClientStore struct {
	KeyStore
	TrustedCertsStore
	ProfileStore
}

// NewClientStore creates a new ClientStore using the provided partial stores.
func NewClientStore(ks KeyStore, ns TrustedCertsStore, ps ProfileStore) *ClientStore {
	return &ClientStore{
		KeyStore:          ks,
		TrustedCertsStore: ns,
		ProfileStore:      ps,
	}
}

// NewMemClientStore initializes a FS backed client store.
func NewFSClientStore(dirPath string) (*ClientStore, error) {
	var err error
	dirPath, err = initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logEntry := logrus.WithField(trace.Component, teleport.ComponentKeyStore)
	return &ClientStore{
		KeyStore:          &FSKeyStore{logEntry, dirPath},
		TrustedCertsStore: &FSTrustedCertsStore{logEntry, dirPath},
		ProfileStore:      &FSProfileStore{logEntry, dirPath},
	}, nil
}

// NewMemClientStore initializes an in-memory client store.
func NewMemClientStore() *ClientStore {
	return &ClientStore{
		KeyStore:          NewMemKeyStore(),
		TrustedCertsStore: NewMemTrustedCertsStore(),
		ProfileStore:      NewMemProfileStore(),
	}
}

// NewClientStoreFromIdentityFile creates a new local client store using the given identity file path.
func NewClientStoreFromIdentityFile(identityFile, proxyAddr, clusterName string) (*ClientStore, error) {
	key, err := KeyFromIdentityFile(identityFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyHost, err := utils.Host(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.ProxyHost = proxyHost
	if clusterName != "" {
		key.ClusterName = clusterName
	}

	keyStore := NewMemClientStore()

	// Save the temporary profile into the key store.
	profile := &profile.Profile{
		WebProxyAddr: proxyAddr,
		SiteName:     key.ClusterName,
		Username:     key.Username,
	}
	if err := keyStore.SaveProfile(profile, true); err != nil {
		return nil, trace.Wrap(err)
	}

	// Preload the client key from the agent.
	key.KeyIndex = KeyIndex{
		ProxyHost:   proxyHost,
		ClusterName: key.ClusterName,
		Username:    key.Username,
	}
	if err := keyStore.AddKey(key); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := keyStore.SaveTrustedCerts(key.ProxyHost, key.TrustedCerts); err != nil {
		return nil, trace.Wrap(err)
	}

	return keyStore, nil
}

// AddKey adds the given key to the key store. The key's trusted certificates are
// also added the the trusted certs store.
func (s *ClientStore) AddKey(key *Key) error {
	if err := s.KeyStore.AddKey(key); err != nil {
		return trace.Wrap(err)
	}
	if err := s.TrustedCertsStore.SaveTrustedCerts(key.ProxyHost, key.TrustedCerts); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetKey gets the requested key with the requested certificates. The key will also
// be populated with corresponding trusted certificates.
func (s *ClientStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	key, err := s.KeyStore.GetKey(idx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCerts, err := s.TrustedCertsStore.GetTrustedCerts(idx.ProxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key.TrustedCerts = trustedCerts
	return key, nil
}

// addTrustedHostKeys is a helper function to add ssh host keys directly, rather than through SaveTrustedCerts.
func (s *ClientStore) AddTrustedHostKeys(proxyHost string, clusterName string, hostKeys ...ssh.PublicKey) error {
	var authorizedKeys [][]byte
	for _, hostKey := range hostKeys {
		authorizedKeys = append(authorizedKeys, ssh.MarshalAuthorizedKey(hostKey))
	}
	err := s.SaveTrustedCerts(proxyHost, []auth.TrustedCerts{
		{
			ClusterName:    clusterName,
			AuthorizedKeys: authorizedKeys,
		},
	})
	return trace.Wrap(err)
}

// ReadProfileStatus returns the profile status for the given profile name.
// If no profile name is provided, return the current profile.
func (s *ClientStore) ReadProfileStatus(profileName string) (*ProfileStatus, error) {
	var err error
	if profileName == "" {
		profileName, err = s.CurrentProfile()
		if err != nil {
			return nil, trace.BadParameter("no profile provided and no current profile")
		}
	} else {
		// remove ports from proxy host, because profile name is stored by host name
		profileName, err = utils.Host(profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	profile, err := s.GetProfile(profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	idx := KeyIndex{
		ProxyHost:   profileName,
		ClusterName: profile.SiteName,
		Username:    profile.Username,
	}
	key, err := s.GetKey(idx, WithAllCerts...)
	if err != nil {
		if trace.IsNotFound(err) {
			// If we can't find a key to match the profile, return a partial status. This
			// is used for some superficial functions `tsh logout` and `tsh status`.
			return &ProfileStatus{
				Name: profileName,
				Dir:  profile.Dir,
				ProxyURL: url.URL{
					Scheme: "https",
					Host:   profile.WebProxyAddr,
				},
				Username:    profile.Username,
				Cluster:     profile.SiteName,
				KubeEnabled: profile.KubeProxyAddr != "",
				// Set ValidUntil to now to show that the keys are not available.
				ValidUntil: time.Now(),
			}, nil
		}
		return nil, trace.Wrap(err)
	}

	_, onDisk := s.KeyStore.(*FSKeyStore)

	return profileStatusFromKey(key, profileOptions{
		ProfileName:   profileName,
		ProfileDir:    profile.Dir,
		WebProxyAddr:  profile.WebProxyAddr,
		Username:      profile.Username,
		SiteName:      profile.SiteName,
		KubeProxyAddr: profile.KubeProxyAddr,
		IsVirtual:     !onDisk,
	})
}

// FullProfileStatus returns the name of the current profile with a
// a list of all active profile statuses.
func (s *ClientStore) FullProfileStatus() (*ProfileStatus, []*ProfileStatus, error) {
	currentProfileName, err := s.CurrentProfile()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	currentProfile, err := s.ReadProfileStatus(currentProfileName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	profileNames, err := s.ListProfiles()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var profiles []*ProfileStatus
	for _, profileName := range profileNames {
		if profileName == currentProfileName {
			// already loaded this one
			continue
		}
		status, err := s.ReadProfileStatus(profileName)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		profiles = append(profiles, status)
	}

	return currentProfile, profiles, nil
}

// noClientStore is a ClientStore representing the absence of a ClientStore.
// All methods return errors. This exists to avoid nil checking everywhere in
// LocalKeyAgent and prevent nil pointer panics.
type noClientStore struct{}

func newNoClientStore() *ClientStore {
	return &ClientStore{noClientStore{}, noClientStore{}, noClientStore{}}
}

var errNoClientStore = trace.NotFound("there is no client store")

func (noClientStore) CurrentProfile() (string, error) {
	return "", errNoClientStore
}
func (noClientStore) ListProfiles() ([]string, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetProfile(profileName string) (*profile.Profile, error) {
	return nil, errNoClientStore
}
func (noClientStore) SaveProfile(*profile.Profile, bool) error {
	return errNoClientStore
}
func (noClientStore) AddKey(key *Key) error {
	return errNoClientStore
}
func (noClientStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	return nil, errNoClientStore
}
func (noClientStore) DeleteKey(idx KeyIndex) error {
	return errNoClientStore
}
func (noClientStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	return errNoClientStore
}
func (noClientStore) DeleteKeys() error {
	return errNoClientStore
}
func (noClientStore) SaveTrustedCerts(proxyHost string, cas []auth.TrustedCerts) error {
	return errNoClientStore
}
func (noClientStore) GetTrustedCerts(proxyHost string) ([]auth.TrustedCerts, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetTrustedHostKeys(clusterNames ...string) ([]ssh.PublicKey, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetSSHCertificates(proxyHost, username string) ([]*ssh.Certificate, error) {
	return nil, errNoClientStore
}
