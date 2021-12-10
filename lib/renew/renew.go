// Package renew contains tools for managing renewable certificates.
package renew

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	trace.Component: teleport.ComponentTSH,
})

type DestinationType string

const (
	// DestinationDir is the destination for certificates stored in a
	// directory on the local filesystem.
	DestinationDir DestinationType = "dir"
)

const (
	// TLSCertKey is the name under which TLS certificates exist in a destination.
	TLSCertKey = "tlscert"

	// TLSCertKey is the name under which SSH certificates exist in a destination.
	SSHCertKey = "sshcert"

	// SSHCACertsKey is the name under which SSH CA certificates exist in a destination.
	SSHCACertsKey = "sshcacerts"

	// TLSCACertsKey is the name under which SSH CA certificates exist in a destination.
	TLSCACertsKey = "tlscacerts"

	// PrivateKeyKey is the name under which the private key exists in a destination.
	// The same private key is used for SSH and TLS certificates.
	PrivateKeyKey = "key"

	// PublicKeyKey is the ssh public key, required for successful SSH connections.
	PublicKeyKey = "key.pub"

	// MetadataKey is the name under which additional metadata exists in a destination.
	MetadataKey = "meta"
)

// DestinationSpec specifies where to place certificates acquired and renewed by tbot.
type DestinationSpec struct {
	Type     DestinationType
	Location string
}

// Destination can persist renewable certificates.
type Destination interface { // TODO: make this the store
	TLSConfig() (*tls.Config, error)

	HostID() (string, error)

	Write(name string, data []byte) error
	Read(name string) ([]byte, error)
}

func NewDestination(d *DestinationSpec) (Destination, error) {
	switch d.Type {
	case DestinationDir:
		return &destinationDir{dir: d.Location}, nil
	default:
		return nil, trace.BadParameter("invalid destination type %v", d.Type)
	}
}

func ParseDestinationSpec(s string) (*DestinationSpec, error) {
	i := strings.Index(s, ":")
	if i == -1 || i == len(s)-1 {
		return nil, fmt.Errorf("invalid destination %v, must be of the form type:location", s)
	}

	var typ DestinationType

	switch t := s[:i]; t {
	case string(DestinationDir):
		typ = DestinationType(t)
	default:
		return nil, fmt.Errorf("invalid destination type %v", t)
	}

	return &DestinationSpec{
		Type:     typ,
		Location: s[i+1:],
	}, nil
}

func SaveIdentity(id *Identity, d Destination) error {
	for _, data := range []struct {
		name string
		data []byte
	}{
		{TLSCertKey, id.TLSCertBytes},
		{SSHCertKey, id.CertBytes},
		{TLSCACertsKey, bytes.Join(id.TLSCACertsBytes, []byte("$"))},
		{SSHCACertsKey, bytes.Join(id.SSHCACertBytes, []byte("\n"))},
		{PrivateKeyKey, id.KeyBytes},
		{PublicKeyKey, id.SSHPublicKeyBytes},
		//{MetadataKey, []byte(id.ID.HostUUID)},
	} {
		log.Debugf("Writing %s", data.name)
		if err := d.Write(data.name, data.data); err != nil {
			return trace.Wrap(err, "could not write to %v", data.name)
		}
	}
	return nil
}

func LoadIdentity(d Destination) (*Identity, error) {
	// TODO: encode the whole thing using the identityfile package?
	var key, sshPublicKey, tlsCA, sshCA []byte
	var certs proto.Certs
	var err error

	for _, item := range []struct {
		name string
		out  *[]byte
	}{
		{TLSCertKey, &certs.TLS},
		{SSHCertKey, &certs.SSH},
		{TLSCACertsKey, &tlsCA},
		{SSHCACertsKey, &sshCA},
		{PrivateKeyKey, &key},
		{PublicKeyKey, &sshPublicKey},
	} {
		*item.out, err = d.Read(item.name)
		if err != nil {
			return nil, trace.Wrap(err, "could not read %v", item.name)
		}
	}

	certs.SSHCACerts = bytes.Split(sshCA, []byte("\n"))
	certs.TLSCACerts = bytes.Split(tlsCA, []byte("$"))

	log.Debugf("Loaded %d SSH CA certs and %d TLS CA certs", len(certs.SSHCACerts), len(certs.TLSCACerts))

	return ReadIdentityFromKeyPair(key, sshPublicKey, &certs)
}
