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
	"encoding/json"
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

const (
	HostCert = "host"
	UserCert = "user"
)

type CAService struct {
	backend backend.Backend
}

func NewCAService(backend backend.Backend) *CAService {
	return &CAService{backend}
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

	return certs, nil
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
