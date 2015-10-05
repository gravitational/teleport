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

// UpsertUserCA upserts the user certificate authority keys in OpenSSH authorized_keys format
func (s *CAService) UpsertUserCA(ca CA) error {
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
	return err
}

// GetCA returns private, public key and certificate for user CA
func (s *CAService) GetUserCA() (*CA, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "userca")
	if err != nil {
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca, nil
}

// GetUserCAPub returns the user certificate authority public key
func (s *CAService) GetUserCAPub() ([]byte, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "userca")
	if err != nil {
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return ca.Pub, nil
}

func (s *CAService) UpsertRemoteCert(rc RemoteCert,
	ttl time.Duration) error {
	if rc.Type != HostCert && rc.Type != UserCert {
		return trace.Errorf("Unknown certificate type '", rc.Type, "'")
	}

	err := s.backend.UpsertVal(
		[]string{"certs", rc.Type, "hosts", rc.FQDN},
		rc.ID, rc.Value, ttl,
	)

	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return err
}

//GetRemoteCerts returns remote certificates with given type and fqdn.
//If fqdn is empty, it returns all certificates with given type
func (s *CAService) GetRemoteCerts(ctype string,
	fqdn string) ([]RemoteCert, error) {

	if ctype != HostCert && ctype != UserCert {
		log.Errorf("Unknown certificate type '" + ctype + "'")
		return nil, trace.Errorf("Unknown certificate type '" + ctype + "'")
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
				return nil, trace.Wrap(err)
			}
			certs[i].Value = value
		}
		return certs, nil
	} else {
		FQDNs, err := s.backend.GetKeys([]string{"certs", ctype,
			"hosts"})
		if err != nil {
			log.Errorf(err.Error())
			return nil, err
		}
		allCerts := make([]RemoteCert, 0)
		for _, f := range FQDNs {
			certs, err := s.GetRemoteCerts(ctype, f)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			allCerts = append(allCerts, certs...)
		}
		return allCerts, nil
	}

}

func (s *CAService) DeleteRemoteCert(ctype, fqdn, id string) error {
	if ctype != HostCert && ctype != UserCert {
		log.Errorf("Unknown certificate type '" + ctype + "'")
		return trace.Errorf("Unknown certificate type '" + ctype + "'")
	}

	err := s.backend.DeleteKey(
		[]string{"certs", ctype, "hosts", fqdn},
		id,
	)

	return err
}

// UpsertHostCA upserts host certificate authority keys in OpenSSH authorized_keys format
func (s *CAService) UpsertHostCA(ca CA) error {
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
	return err
}

// GetHostCA returns private, public key and certificate for host CA
func (s *CAService) GetHostCA() (*CA, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "hostca")
	if err != nil {
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &ca, nil
}

// GetHostCACert returns the host certificate authority certificate
func (s *CAService) GetHostCAPub() ([]byte, error) {
	val, err := s.backend.GetVal([]string{"ca"}, "hostca")
	if err != nil {
		return nil, err
	}

	var ca CA
	err = json.Unmarshal(val, &ca)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
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
