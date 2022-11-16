package license

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/json"
	"errors"
)

var payloadOID = asn1.ObjectIdentifier{2, 5, 42}

func PayloadFromCert(cert *x509.Certificate) []byte {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(payloadOID) {
			return ext.Value
		}
	}
	return nil
}

func LicenseFromCert(cert *x509.Certificate) (License, error) {
	var l License
	if err := json.Unmarshal(PayloadFromCert(cert), &l); err != nil {
		return License{}, err
	}
	if l.Metadata.Name == "" {
		return License{}, errors.New("missing name")
	}
	return l, nil
}

type License struct {
	Metadata Metadata `json:"metadata"`
	Spec     Spec     `json:"spec"`
}

type Metadata struct {
	Name string `json:"name"`
}

type Spec struct {
	AccountID string `json:"account_id"`
	Cloud     bool   `json:"cloud"`
}

// AccountID returns an ID that represents the account associated with the
// license. The account associated to a license with an empty account_id is
// assumed to be equal to the license name.
func (l License) AccountID() string {
	if l.Spec.AccountID != "" {
		return l.Spec.AccountID
	}
	return l.Metadata.Name
}
