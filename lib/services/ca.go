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
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const (
	HostCert = "host"
	UserCert = "user"
)

type CAService struct {
	backend backend.Backend
	IsCache bool
}

func NewCAService(backend backend.Backend) *CAService {
	return &CAService{backend: backend}
}

// UpsertUserCertificateAuthority upserts the user certificate authority keys in OpenSSH authorized_keys format
func (s *CAService) UpsertUserCertificateAuthority(ca LocalCertificateAuthority) error {
	ca.Type = UserCert
	out, err := json.Marshal(ca)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{"ca"}, "userca", out, 0)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return nil
}

// GetCertificateAuthority returns private, public key and certificate for user CertificateAuthority
func (s *CAService) GetUserPrivateCertificateAuthority() (*LocalCertificateAuthority, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "userca")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var ca LocalCertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca, nil
}

// GetUserCertificateAuthority returns the user certificate authority public key
func (s *CAService) GetUserCertificateAuthority() (*CertificateAuthority, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "userca")
	if err != nil {
		return nil, err
	}

	var ca LocalCertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca.CertificateAuthority, nil
}

func (s *CAService) UpsertRemoteCertificate(cert CertificateAuthority, ttl time.Duration) error {
	if cert.Type != HostCert && cert.Type != UserCert {
		return trace.Errorf("unknown certificate type '%v'", cert.Type)
	}

	err := s.backend.UpsertVal(
		[]string{"certs", cert.Type, "hosts", cert.DomainName},
		cert.ID, cert.PublicKey, ttl,
	)

	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return nil
}

//GetRemoteCertificates returns remote certificates with given type and domain.
//If domainName is empty, it returns all certificates with given type
func (s *CAService) GetRemoteCertificates(certType string,
	domainName string) ([]CertificateAuthority, error) {

	if certType != HostCert && certType != UserCert {
		log.Errorf("Unknown certificate type '" + certType + "'")
		return nil, trace.Errorf("Unknown certificate type '" + certType + "'")
	}

	if domainName != "" {
		IDs, err := s.backend.GetKeys([]string{"certs", certType,
			"hosts", domainName})
		if err != nil {
			log.Errorf(err.Error())
			return nil, trace.Wrap(err)
		}
		certs := make([]CertificateAuthority, len(IDs))
		for i, id := range IDs {
			certs[i].Type = certType
			certs[i].DomainName = domainName
			certs[i].ID = id
			value, err := s.backend.GetVal(
				[]string{"certs", certType, "hosts", domainName}, id)
			if err != nil {
				log.Errorf(err.Error())
				return nil, trace.Wrap(err)
			}
			certs[i].PublicKey = value
		}
		return certs, nil
	} else {
		DomainNames, err := s.backend.GetKeys([]string{"certs", certType,
			"hosts"})
		if err != nil {
			log.Errorf(err.Error())
			return nil, trace.Wrap(err)
		}
		allCerts := make([]CertificateAuthority, 0)
		for _, f := range DomainNames {
			certs, err := s.GetRemoteCertificates(certType, f)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			allCerts = append(allCerts, certs...)
		}
		return allCerts, nil
	}

}

func (s *CAService) DeleteRemoteCertificate(certType, domainName, id string) error {
	if certType != HostCert && certType != UserCert {
		log.Errorf("Unknown certificate type '" + certType + "'")
		return trace.Errorf("Unknown certificate type '" + certType + "'")
	}

	err := s.backend.DeleteKey(
		[]string{"certs", certType, "hosts", domainName},
		id,
	)

	if err != nil {
		return trace.Wrap(err)
	} else {
		return nil
	}
}

// UpsertHostCertificateAuthority upserts host certificate authority keys in OpenSSH authorized_keys format
func (s *CAService) UpsertHostCertificateAuthority(ca LocalCertificateAuthority) error {
	ca.Type = HostCert
	out, err := json.Marshal(ca)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{"ca"}, "hostca", out, 0)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return nil
}

// GetHostPrivateCertificateAuthority returns private, public key and certificate for host CA
func (s *CAService) GetHostPrivateCertificateAuthority() (*LocalCertificateAuthority, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "hostca")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var ca LocalCertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca, nil
}

// GetHostCertificateAuthority returns the host certificate authority certificate
func (s *CAService) GetHostCertificateAuthority() (*CertificateAuthority, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "hostca")
	if err != nil {
		return nil, err
	}

	var ca LocalCertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca.CertificateAuthority, nil
}

func (s *CAService) GetTrustedCertificates(certType string) ([]CertificateAuthority, error) {
	certs := []CertificateAuthority{}
	remoteCerts, err := s.GetRemoteCertificates(certType, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs = append(certs, remoteCerts...)

	if !s.IsCache {
		if certType == "" || certType == UserCert {
			userCert, err := s.GetUserCertificateAuthority()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			certs = append(certs, *userCert)
		}

		if certType == "" || certType == HostCert {
			hostCert, err := s.GetHostCertificateAuthority()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			certs = append(certs, *hostCert)
		}
	}

	return certs, nil
}

func (s *CAService) GetCertificateID(certType string, key ssh.PublicKey) (ID string, found bool, e error) {
	certs, err := s.GetTrustedCertificates(certType)
	if err != nil {
		return "", false, trace.Wrap(err)
	}
	for _, cert := range certs {
		parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(cert.PublicKey)
		if err != nil {
			log.Errorf("failed to parse CA public key '%v', err: %v",
				string(cert.PublicKey), err)
			continue
		}
		if sshutils.KeysEqual(key, parsedKey) {
			return cert.ID, true, nil
		}
	}
	return "", false, nil
}

type LocalCertificateAuthority struct {
	CertificateAuthority `json:"public"`
	PrivateKey           []byte `json:"private_key"`
}

type CertificateAuthority struct {
	Type       string `json:"type" yaml:"type"`
	ID         string `json:"id" yaml:"id"`
	DomainName string `json:"domain_name" yaml:"domain_name" env:"domain_name"`
	PublicKey  []byte `json:"public_key" yaml:"public_key" env:"public_key"`
}

// Marshall user mapping into string
func UserMappingHash(certificateID, teleportUser, osUser string) (string, error) {
	jsonString, err := json.Marshal([]string{certificateID, teleportUser, osUser})
	if err != nil {
		return "", trace.Wrap(err)
	}
	b64str := base64.StdEncoding.EncodeToString(jsonString)
	return string(b64str), nil
}

// Unmarshall user mapping from string
func ParseUserMappingHash(hash string) (certificateID, teleportUser, osUser string, e error) {
	jsonString, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		return "", "", "", trace.Wrap(err)
	}
	var values []string
	err = json.Unmarshal(jsonString, &values)
	if err != nil {
		return "", "", "", trace.Wrap(err)
	}
	if len(values) != 3 {
		return "", "", "", trace.Errorf("parsing error")
	}
	return values[0], values[1], values[2], nil
}

func (s *CAService) UpsertUserMapping(certificateID, teleportUser, osUser string, ttl time.Duration) error {
	hash, err := UserMappingHash(certificateID, teleportUser, osUser)
	err = s.backend.UpsertVal([]string{"usermap"}, hash, []byte("ok"), ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *CAService) UserMappingExists(certificateID, teleportUser, osUser string) (bool, error) {
	hash, err := UserMappingHash(certificateID, teleportUser, osUser)
	val, err := s.backend.GetVal([]string{"usermap"}, hash)
	if err != nil {
		return false, nil
	}
	if string(val) != "ok" {
		return false, trace.Errorf("Value should be 'ok'")
	}
	return true, nil
}

func (s *CAService) DeleteUserMapping(certificateID, teleportUser, osUser string) error {
	hash, err := UserMappingHash(certificateID, teleportUser, osUser)
	err = s.backend.DeleteKey([]string{"usermap"}, hash)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *CAService) GetAllUserMappings() (hashes []string, e error) {
	hashes, err := s.backend.GetKeys([]string{"usermap"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return hashes, nil
}

func (s *CAService) UpdateUserMappings(hashes []string, ttl time.Duration) error {
	for _, hash := range hashes {
		err := s.backend.UpsertVal([]string{"usermap"}, hash, []byte("ok"), ttl)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
