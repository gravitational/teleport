package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/log"
	"github.com/gravitational/teleport/backend"
)

type CAService struct {
	backend backend.Backend
}

// UpsertUserCA upserts the user certificate authority keys in OpenSSH authorized_keys format
func (s *CAService) UpsertUserCA(ca CA) error {
	out, err := json.Marshal(ca)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}
	err = s.backend.UpsertVal([]string{}, "userca", out, 0)
	if err != nil {
		log.Errorf(err.Error())
	}
	return err
}

// GetCA returns private, public key and certificate for user CA
func (s *CAService) GetUserCA() (*CA, error) {
	val, err := s.backend.GetVal([]string{}, "userca")
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return &ca, nil
}

// GetUserCAPub returns the user certificate authority public key
func (s *CAService) GetUserCAPub() ([]byte, error) {
	val, err := s.backend.GetVal([]string{}, "userca")
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return ca.Pub, nil
}

func (s *CAService) UpsertRemoteCert(rc RemoteCert,
	ttl time.Duration) error {
	if rc.Type != "host" && rc.Type != "user" {
		return fmt.Errorf("Unknown certificate type '", rc.Type, "'")
	}

	err := s.backend.UpsertVal(
		[]string{"certs", rc.Type, "hosts", rc.FQDN},
		rc.ID, rc.Value, ttl,
	)

	if err != nil {
		log.Errorf(err.Error())
	}
	return err
}

//GetRemoteCerts returns remote certificates with given type and fqdn.
//If fqdn is empty, it returns all certificates with given type
func (s *CAService) GetRemoteCerts(ctype string,
	fqdn string) ([]RemoteCert, error) {

	if ctype != "host" && ctype != "user" {
		log.Errorf("Unknown certificate type '" + ctype + "'")
		return nil, fmt.Errorf("Unknown certificate type '" + ctype + "'")
	}

	if fqdn != "" {
		IDs, err := s.backend.GetKeys([]string{"certs", ctype,
			"hosts", fqdn})
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
		certs := make([]RemoteCert, len(IDs))
		for i, id := range IDs {
			certs[i].Type = ctype
			certs[i].FQDN = fqdn
			certs[i].ID = id
			value, err := s.backend.GetVal(
				[]string{"certs", ctype, "hosts", fqdn}, id)
			if err != nil {
				log.Errorf(err.Error())
				return nil, err
			}
			certs[i].Value = value
		}
		return certs, nil
	} else {
		FQDNs, err := s.backend.GetKeys([]string{"certs", ctype,
			"hosts", fqdn})
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
		allCerts := make([]RemoteCert, 0)
		for _, f := range FQDNs {
			certs, err := s.GetRemoteCerts(ctype, f)
			if err != nil {
				return nil, err
			}
			allCerts = append(allCerts, certs...)
		}
		return allCerts, nil
	}

}

func (s *CAService) DeleteRemoteCert(ctype, fqdn, id string) error {
	return s.backend.DeleteKey(
		[]string{"certs", ctype, "hosts", fqdn},
		id,
	)
}

// UpsertHostCA upserts host certificate authority keys in OpenSSH authorized_keys format
func (s *CAService) UpsertHostCA(ca CA) error {
	out, err := json.Marshal(ca)
	if err != nil {
		log.Errorf(err.Error())
		return err
	}
	err = s.backend.UpsertVal([]string{}, "hostca", out, 0)
	if err != nil {
		log.Errorf(err.Error())
	}
	return err
}

// GetHostCA returns private, public key and certificate for host CA
func (s *CAService) GetHostCA() (*CA, error) {
	val, err := s.backend.GetVal([]string{}, "hostca")
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return &ca, nil
}

// GetHostCACert returns the host certificate authority certificate
func (s *CAService) GetHostCAPub() ([]byte, error) {
	val, err := s.backend.GetVal([]string{}, "hostca")
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, err
	}

	return ca.Pub, nil
}

// CA is a set of private and public keys
type CA struct {
	Pub  []byte `json:"pub"`
	Priv []byte `json:"priv"`
}

type RemoteCert struct {
	Type  string
	ID    string
	FQDN  string
	Value []byte
}
