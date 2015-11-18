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

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/lib/backend"
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
func (s *CAService) UpsertUserCertificateAuthority(ca CertificateAuthority) error {
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
func (s *CAService) GetUserCertificateAuthority() (*CertificateAuthority, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "userca")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var ca CertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca, nil
}

// GetUserPublicCertificate returns the user certificate authority public key
func (s *CAService) GetUserPublicCertificate() (PublicCertificate, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "userca")
	if err != nil {
		return PublicCertificate{}, err
	}

	var ca CertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return PublicCertificate{}, trace.Wrap(err)
	}

	return ca.PublicCertificate, nil
}

func (s *CAService) UpsertRemoteCertificate(cert PublicCertificate, ttl time.Duration) error {
	if cert.Type != HostCert && cert.Type != UserCert {
		return trace.Errorf("unknown certificate type '%v'", cert.Type)
	}

	err := s.backend.UpsertVal(
		[]string{"certs", cert.Type, "hosts", cert.FQDN},
		cert.ID, cert.PubValue, ttl,
	)

	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return nil
}

//GetRemoteCertificates returns remote certificates with given type and fqdn.
//If fqdn is empty, it returns all certificates with given type
func (s *CAService) GetRemoteCertificates(certType string,
	fqdn string) ([]PublicCertificate, error) {

	if certType != HostCert && certType != UserCert {
		log.Errorf("Unknown certificate type '" + certType + "'")
		return nil, trace.Errorf("Unknown certificate type '" + certType + "'")
	}

	if fqdn != "" {
		IDs, err := s.backend.GetKeys([]string{"certs", certType,
			"hosts", fqdn})
		if err != nil {
			log.Errorf(err.Error())
			return nil, trace.Wrap(err)
		}
		certs := make([]PublicCertificate, len(IDs))
		for i, id := range IDs {
			certs[i].Type = certType
			certs[i].FQDN = fqdn
			certs[i].ID = id
			value, err := s.backend.GetVal(
				[]string{"certs", certType, "hosts", fqdn}, id)
			if err != nil {
				log.Errorf(err.Error())
				return nil, trace.Wrap(err)
			}
			certs[i].PubValue = value
		}
		return certs, nil
	} else {
		FQDNs, err := s.backend.GetKeys([]string{"certs", certType,
			"hosts"})
		if err != nil {
			log.Errorf(err.Error())
			return nil, trace.Wrap(err)
		}
		allCerts := make([]PublicCertificate, 0)
		for _, f := range FQDNs {
			certs, err := s.GetRemoteCertificates(certType, f)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			allCerts = append(allCerts, certs...)
		}
		return allCerts, nil
	}

}

func (s *CAService) DeleteRemoteCertificate(certType, fqdn, id string) error {
	if certType != HostCert && certType != UserCert {
		log.Errorf("Unknown certificate type '" + certType + "'")
		return trace.Errorf("Unknown certificate type '" + certType + "'")
	}

	err := s.backend.DeleteKey(
		[]string{"certs", certType, "hosts", fqdn},
		id,
	)

	if err != nil {
		return trace.Wrap(err)
	} else {
		return nil
	}
}

// UpsertHostCertificateAuthority upserts host certificate authority keys in OpenSSH authorized_keys format
func (s *CAService) UpsertHostCertificateAuthority(ca CertificateAuthority) error {
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

// GetHostCertificateAuthority returns private, public key and certificate for host CA
func (s *CAService) GetHostCertificateAuthority() (*CertificateAuthority, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "hostca")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var ca CertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca, nil
}

// GetHostPublicCertificate returns the host certificate authority certificate
func (s *CAService) GetHostPublicCertificate() (PublicCertificate, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "hostca")
	if err != nil {
		return PublicCertificate{}, err
	}

	var ca CertificateAuthority
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return PublicCertificate{}, trace.Wrap(err)
	}

	return ca.PublicCertificate, nil
}

func (s *CAService) GetTrustedCertificates(certType string) ([]PublicCertificate, error) {
	certs := []PublicCertificate{}
	remoteCerts, err := s.GetRemoteCertificates(certType, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs = append(certs, remoteCerts...)

	if certType == "" || certType == UserCert {
		userCert, err := s.GetUserPublicCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs = append(certs, userCert)
	}

	if certType == "" || certType == HostCert {
		hostCert, err := s.GetHostPublicCertificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs = append(certs, hostCert)
	}

	return certs, nil
}

type CertificateAuthority struct {
	PublicCertificate `json: "pub"`
	PrivValue         []byte `json:"priv"`
}

type PublicCertificate struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	FQDN     string `json:"fqdn"`
	PubValue []byte `json:"value"`
}
