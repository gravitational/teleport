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

func registerUsingTokenRequestToClientInitMessage(req *types.RegisterUsingTokenRequest, joinMethod string) (*messages.ClientInit, error) {
	rawTLSPub, _ := pem.Decode(req.PublicTLSKey)
	if rawTLSPub == nil {
		return nil, trace.BadParameter("failed to decode PublicTLSKey from PEM")
	}
	sshPub, _, _, _, err := ssh.ParseAuthorizedKey(req.PublicSSHKey)
	if err != nil {
		return nil, trace.Wrap(err, "parsing PublicSSHKey from authorized_keys format")
	}
	clientInit := &messages.ClientInit{
		JoinMethod:   &joinMethod,
		TokenName:    req.Token,
		SystemRole:   req.Role.String(),
		PublicTLSKey: rawTLSPub.Bytes,
		PublicSSHKey: sshPub.Marshal(),
	}
	if clientInit.SystemRole == types.RoleBot.String() {
		clientInit.BotParams = &messages.BotParams{
			Expires: req.Expires,
		}
	}
	return clientInit, nil
}

func protoCertsFromResultMessage(result *messages.Result) (*proto.Certs, error) {
	sshCert, err := sshPubWireFormatToAuthorizedKey(result.SSHCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sshCAKeys, err := sshPubWireFormatsToAuthorizedKeys(result.SSHCAKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.Certs{
		TLS:        pemEncodeTLSCert(result.TLSCert),
		TLSCACerts: pemEncodeTLSCerts(result.TLSCACerts),
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
