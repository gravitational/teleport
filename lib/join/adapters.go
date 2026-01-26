// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package join

import (
	"encoding/pem"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/internal/messages"
)

func clientInitFromRegisterUsingTokenRequest(req *types.RegisterUsingTokenRequest, joinMethod string) *messages.ClientInit {
	return &messages.ClientInit{
		JoinMethod: &joinMethod,
		TokenName:  req.Token,
		SystemRole: req.Role.String(),
	}
}

func clientParamsFromRegisterUsingTokenRequest(req *types.RegisterUsingTokenRequest) (*messages.ClientParams, error) {
	rawTLSPub, _ := pem.Decode(req.PublicTLSKey)
	if rawTLSPub == nil {
		return nil, trace.BadParameter("failed to decode PublicTLSKey from PEM")
	}
	sshPub, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicSSHKey)
	if err != nil {
		return nil, trace.Wrap(err, "parsing PublicSSHKey from authorized_keys format")
	}
	publicKeys := messages.PublicKeys{
		PublicTLSKey: rawTLSPub.Bytes,
		PublicSSHKey: sshPub.Marshal(),
	}
	var clientParams messages.ClientParams
	if req.Role == types.RoleBot {
		clientParams.BotParams = &messages.BotParams{
			PublicKeys: publicKeys,
			Expires:    req.Expires,
		}
	} else {
		clientParams.HostParams = &messages.HostParams{
			PublicKeys:           publicKeys,
			HostName:             req.NodeName,
			AdditionalPrincipals: req.AdditionalPrincipals,
			DNSNames:             req.DNSNames,
		}
	}
	return &clientParams, nil
}

func protoCertsFromCertificates(certs messages.Certificates) (*proto.Certs, error) {
	sshCert, err := sshPubWireFormatToAuthorizedKey(certs.SSHCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshCAKeys, err := sshPubWireFormatsToAuthorizedKeys(certs.SSHCAKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.Certs{
		TLS:        pemEncodeTLSCert(certs.TLSCert),
		TLSCACerts: pemEncodeTLSCerts(certs.TLSCACerts),
		SSH:        sshCert,
		SSHCACerts: sshCAKeys, // SSHCACerts is a misnomer, they're just public keys.
	}, nil
}

func pemEncodeTLSCerts(rawCerts [][]byte) [][]byte {
	out := make([][]byte, len(rawCerts))
	for i, rawCert := range rawCerts {
		out[i] = pemEncodeTLSCert(rawCert)
	}
	return out
}

func pemEncodeTLSCert(rawCert []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCert,
	})
}

func sshPubWireFormatsToAuthorizedKeys(wireFormats [][]byte) ([][]byte, error) {
	out := make([][]byte, len(wireFormats))
	for i, wireFormat := range wireFormats {
		var err error
		out[i], err = sshPubWireFormatToAuthorizedKey(wireFormat)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return out, nil
}

func sshPubWireFormatToAuthorizedKey(wireFormat []byte) ([]byte, error) {
	pub, err := ssh.ParsePublicKey(wireFormat)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh.MarshalAuthorizedKey(pub), nil
}
