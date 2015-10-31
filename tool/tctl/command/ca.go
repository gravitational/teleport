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
package command

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/buger/goterm"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

func (cmd *Command) GenerateKeyPair(privateKeyPath, publicKeyPath, passphrase string) error {
	priv, pub, err := native.New().GenerateKeyPair(passphrase)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := ioutil.WriteFile(privateKeyPath, priv, 0600); err != nil {
		return trace.Wrap(err)
	}
	if err := ioutil.WriteFile(publicKeyPath, pub, 0666); err != nil {
		return trace.Wrap(err)
	}
	cmd.printOK("Public and private keys have been written")
	return nil
}

func (cmd *Command) ResetHostCA(confirm bool) {
	if !confirm && !cmd.confirm("Reseting private and public keys for Host CA. This will invalidate all signed host certs. Continue?") {
		cmd.printError(fmt.Errorf("aborted by user"))
		return
	}
	if err := cmd.client.ResetHostCA(); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("CA keys have been regenerated")
}

func (cmd *Command) GetHostCAPub() {
	key, err := cmd.client.GetHostCAPub()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Host CA Key")
	fmt.Fprintf(cmd.out, string(key))
}

func (cmd *Command) ResetUserCA(confirm bool) {
	if !confirm && !cmd.confirm("Reseting private and public keys for User CA. This will invalidate all signed user certs. Continue?") {
		cmd.printError(fmt.Errorf("aborted by user"))
		return
	}
	if err := cmd.client.ResetUserCA(); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("CA keys have been regenerated")
}

func (cmd *Command) GetUserCAPub() {
	key, err := cmd.client.GetUserCAPub()
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("User CA Key")
	fmt.Fprintf(cmd.out, string(key))
}

func (cmd *Command) UpsertRemoteCert(id, fqdn, certType, path string, ttl time.Duration) {
	val, err := cmd.readInput(path)
	if err != nil {
		cmd.printError(err)
		return
	}
	cert := services.RemoteCert{
		FQDN:  fqdn,
		Type:  certType,
		ID:    id,
		Value: val,
	}
	if err := cmd.client.UpsertRemoteCert(cert, ttl); err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("Remote cert have been upserted")
}

func (cmd *Command) GetRemoteCerts(fqdn, certType string) {
	certs, err := cmd.client.GetRemoteCerts(certType, fqdn)
	if err != nil {
		cmd.printError(err)
		return
	}
	fmt.Fprintf(cmd.out, remoteCertsView(certs))
}

func (cmd *Command) DeleteRemoteCert(id, fqdn, certType string) {
	err := cmd.client.DeleteRemoteCert(certType, fqdn, id)
	if err != nil {
		cmd.printError(err)
		return
	}
	cmd.printOK("certificate deleted")
}

func remoteCertsView(certs []services.RemoteCert) string {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	fmt.Fprint(t, "Type\tFQDN\tID\tValue\n")
	if len(certs) == 0 {
		return t.String()
	}
	for _, c := range certs {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n", c.Type, c.FQDN, c.ID, string(c.Value))
	}
	return t.String()
}
