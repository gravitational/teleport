/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"errors"
	"net/url"
	"os"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
)

// Store is a storage interface for client data. Store is made up of three
// partial data stores; KeyStore, TrustedCertsStore, and ProfileStore.
//
// A Store can be made up of partial data stores with different backends. For example,
// when using `tsh --add-keys-to-agent=only`, Store will be made up of an in-memory
// key store and an FS (~/.tsh) profile and trusted certs store.
type Store struct {
	log *logrus.Entry

	KeyStore
	TrustedCertsStore
	ProfileStore
}

// NewMemClientStore initializes an FS backed client store with the given base dir.
func NewFSClientStore(dirPath string) *Store {
	dirPath = profile.FullProfilePath(dirPath)
	return &Store{
		log:               logrus.WithField(teleport.ComponentKey, teleport.ComponentKeyStore),
		KeyStore:          NewFSKeyStore(dirPath),
		TrustedCertsStore: NewFSTrustedCertsStore(dirPath),
		ProfileStore:      NewFSProfileStore(dirPath),
	}
}

// NewMemClientStore initializes a new in-memory client store.
func NewMemClientStore() *Store {
	return &Store{
		log:               logrus.WithField(teleport.ComponentKey, teleport.ComponentKeyStore),
		KeyStore:          NewMemKeyStore(),
		TrustedCertsStore: NewMemTrustedCertsStore(),
		ProfileStore:      NewMemProfileStore(),
	}
}

// AddKey adds the given key to the key store. The key's trusted certificates are
// added to the trusted certs store.
func (s *Store) AddKey(key *KeyRing) error {
	if err := s.KeyStore.AddKey(key); err != nil {
		return trace.Wrap(err)
	}
	if err := s.TrustedCertsStore.SaveTrustedCerts(key.ProxyHost, key.TrustedCerts); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

var (
	// ErrNoCredentials is returned by the client store when a specific key is not found.
	// This error can be used to determine whether a client should retrieve new credentials,
	// like how it is used with lib/client.RetryWithRelogin.
	ErrNoCredentials = &trace.NotFoundError{Message: "no credentials"}

	// ErrNoProfile is returned by the client store when a specific profile is not found.
	// This error can be used to determine whether a client should retrieve new credentials,
	// like how it is used with lib/client.RetryWithRelogin.
	ErrNoProfile = &trace.NotFoundError{Message: "no profile"}
)

// IsNoCredentialsError returns whether the given error implies that the user should retrieve new credentials.
func IsNoCredentialsError(err error) bool {
	return errors.Is(err, ErrNoCredentials) || errors.Is(err, ErrNoProfile)
}

// GetKey gets the requested key with trusted the requested certificates. The key's
// trusted certs will be retrieved from the trusted certs store. If the key is not
// found or is missing data (certificates, etc.), then an ErrNoCredentials error
// is returned.
func (s *Store) GetKey(idx KeyIndex, opts ...CertOption) (*KeyRing, error) {
	key, err := s.KeyStore.GetKey(idx, opts...)
	if trace.IsNotFound(err) {
		return nil, trace.Wrap(ErrNoCredentials, err.Error())
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCertExpiration, err := key.TeleportTLSCertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Debugf("Teleport TLS certificate valid until %q.", tlsCertExpiration)

	// Validate the SSH certificate.
	if key.Cert != nil {
		if err := key.CheckCert(); err != nil {
			if !utils.IsCertExpiredError(err) {
				return nil, trace.Wrap(err)
			}
		}
	}

	trustedCerts, err := s.TrustedCertsStore.GetTrustedCerts(idx.ProxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key.TrustedCerts = trustedCerts
	return key, nil
}

// AddTrustedHostKeys is a helper function to add ssh host keys directly, rather than through SaveTrustedCerts.
func (s *Store) AddTrustedHostKeys(proxyHost string, clusterName string, hostKeys ...ssh.PublicKey) error {
	var authorizedKeys [][]byte
	for _, hostKey := range hostKeys {
		authorizedKeys = append(authorizedKeys, ssh.MarshalAuthorizedKey(hostKey))
	}
	err := s.SaveTrustedCerts(proxyHost, []authclient.TrustedCerts{
		{
			ClusterName:    clusterName,
			AuthorizedKeys: authorizedKeys,
		},
	})
	return trace.Wrap(err)
}

// ReadProfileStatus returns the profile status for the given profile name.
// If no profile name is provided, return the current profile.
func (s *Store) ReadProfileStatus(profileName string) (*ProfileStatus, error) {
	var err error
	if profileName == "" {
		profileName, err = s.CurrentProfile()
		if err != nil {
			return nil, trace.Wrap(err)
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
		if trace.IsNotFound(err) {
			return nil, trace.Wrap(ErrNoProfile, err.Error())
		}
		return nil, trace.Wrap(err)
	}
	idx := KeyIndex{
		ProxyHost:   profileName,
		ClusterName: profile.SiteName,
		Username:    profile.Username,
	}
	key, err := s.GetKey(idx, WithAllCerts...)
	if err != nil {
		if trace.IsNotFound(err) || trace.IsConnectionProblem(err) {
			// If we can't find a key to match the profile, or can't connect to
			// the key (hardware key), return a partial status. This is used for
			// some superficial functions `tsh logout` and `tsh status`.
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
				ValidUntil:              time.Now(),
				SAMLSingleLogoutEnabled: profile.SAMLSingleLogoutEnabled,
			}, nil
		}
		return nil, trace.Wrap(err)
	}

	_, onDisk := s.KeyStore.(*FSKeyStore)

	return profileStatusFromKey(key, profileOptions{
		ProfileName:             profileName,
		ProfileDir:              profile.Dir,
		WebProxyAddr:            profile.WebProxyAddr,
		Username:                profile.Username,
		SiteName:                profile.SiteName,
		KubeProxyAddr:           profile.KubeProxyAddr,
		SAMLSingleLogoutEnabled: profile.SAMLSingleLogoutEnabled,
		IsVirtual:               !onDisk,
	})
}

// FullProfileStatus returns the name of the current profile with a
// a list of all profile statuses.
func (s *Store) FullProfileStatus() (*ProfileStatus, []*ProfileStatus, error) {
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
			s.log.WithError(err).Warnf("skipping profile %q due to error", profileName)
			continue
		}
		profiles = append(profiles, status)
	}

	return currentProfile, profiles, nil
}

// LoadKeysToKubeFromStore loads the keys for a given teleport cluster and kube cluster from the store.
// It returns the certificate and private key to be used for the kube cluster.
// If the keys are not found, it returns an error.
// This function is used to speed up the credentials loading process since Teleport
// Store transverses the entire store to find the keys. This operation takes a long time
// when the store has a lot of keys and when we call the function multiple times in
// parallel.
// Although this function speeds up the process since it removes all transversals,
// it still has to read 2 different files:
// - $TSH_HOME/keys/$PROXY/$USER-kube/$TELEPORT_CLUSTER/$KUBE_CLUSTER-x509.pem
// - $TSH_HOME/keys/$PROXY/$USER
func LoadKeysToKubeFromStore(profile *profile.Profile, dirPath, teleportCluster, kubeCluster string) ([]byte, []byte, error) {
	fsKeyStore := NewFSKeyStore(dirPath)

	certPath := fsKeyStore.kubeCertPath(KeyIndex{ProxyHost: profile.SiteName, ClusterName: teleportCluster, Username: profile.Username}, kubeCluster)
	kubeCert, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	privKeyPath := fsKeyStore.userKeyPath(KeyIndex{ProxyHost: profile.SiteName, Username: profile.Username})
	privKey, err := os.ReadFile(privKeyPath)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := keys.AssertSoftwarePrivateKey(privKey); err != nil {
		return nil, nil, trace.Wrap(err, "unsupported private key type")
	}
	return kubeCert, privKey, nil
}
