// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/protocol/webauthncbor"
	"github.com/duo-labs/webauthn/protocol/webauthncose"
	"github.com/gravitational/trace"

	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

type fido2Command struct {
	diag   *fido2DiagCommand
	attobj *fido2AttobjCommand
}

func newFIDO2Command(app *kingpin.Application) *fido2Command {
	root := &fido2Command{
		diag:   &fido2DiagCommand{},
		attobj: &fido2AttobjCommand{},
	}

	f2 := app.Command("fido2", "FIDO2 commands").Hidden()

	diag := f2.Command("diag", "Run FIDO2 diagnostics").Hidden()
	root.diag.CmdClause = diag

	attObj := f2.Command("attobj", "Parse a stored attestation object").Hidden()
	attObj.
		Arg("att-obj", "Attestation object encoded in base64 standard or RawURL").
		Required().
		StringVar(&root.attobj.attObjB64)
	root.attobj.CmdClause = attObj

	return root
}

type fido2DiagCommand struct {
	*kingpin.CmdClause
}

func (_ *fido2DiagCommand) run(cf *CLIConf) error {
	diag, err := wancli.FIDO2Diag(cf.Context, os.Stdout)
	// Abort if we got a nil diagnostic, otherwise print as much as we can.
	if diag == nil {
		return trace.Wrap(err)
	}

	fmt.Printf("\nFIDO2 available: %v\n", diag.Available)
	fmt.Printf("Register successful? %v\n", diag.RegisterSuccessful)
	fmt.Printf("Login successful? %v\n", diag.LoginSuccessful)
	if err != nil {
		fmt.Println()
	}

	return trace.Wrap(err)
}

type fido2AttobjCommand struct {
	*kingpin.CmdClause

	attObjB64 string
}

func (c *fido2AttobjCommand) run(_ *CLIConf) error {
	var aoRaw []byte
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawURLEncoding,
	} {
		var err error
		aoRaw, err = enc.DecodeString(c.attObjB64)
		if err == nil {
			break
		}
	}
	if aoRaw == nil {
		return errors.New("failed to decode attestation object")
	}

	ao := &protocol.AttestationObject{}
	if err := webauthncbor.Unmarshal(aoRaw, ao); err != nil {
		return trace.Wrap(err, "attestation object unmarshal")
	}
	if err := ao.AuthData.Unmarshal(ao.RawAuthData); err != nil {
		return trace.Wrap(err, "authdata unmarshal")
	}

	// Print attestation object as JSON.
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ao); err != nil {
		return trace.Wrap(err, "encode attestation object to JSON")
	}

	// Print public key.
	if len(ao.AuthData.AttData.CredentialPublicKey) > 0 {
		pubKey, err := webauthncose.ParsePublicKey(ao.AuthData.AttData.CredentialPublicKey)
		if err == nil {
			fmt.Println("\nAuthData.AttData.public_key:")
			if err := enc.Encode(pubKey); err != nil {
				return trace.Wrap(err, "encode public key")
			}
		}
	}

	// Print attestation certificates.
	if x5c, ok := ao.AttStatement["x5c"]; ok {
		if x5cArray, ok := x5c.([]interface{}); ok {
			for i, certI := range x5cArray {
				certDER, ok := certI.([]byte)
				if !ok {
					continue
				}

				cert, err := x509.ParseCertificate(certDER)
				if err != nil {
					log.WithError(err).Warnf("Failed to parse X.509 from x5c[%v], continuing", i)
					continue
				}

				type niceCert struct {
					Raw     []byte
					Issuer  string
					Subject string
				}

				fmt.Printf("\nattStmt.x509[%v]:\n", i)
				enc.Encode(niceCert{
					Raw:     cert.Raw,
					Issuer:  cert.Issuer.String(),
					Subject: cert.Subject.String(),
				})
			}
		}
	}

	return nil
}
