package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func Register(fqdn, dataDir, token string, servers []utils.NetAddr) error {
	tok, err := readToken(token)
	if err != nil {
		return err
	}
	method, err := NewTokenAuth(fqdn, tok)
	if err != nil {
		return err
	}
	config := &ssh.ClientConfig{
		User: fqdn,
		Auth: method,
	}
	client, err := ssh.Dial(servers[0].Network, servers[0].Addr, config)
	if err != nil {
		return err
	}
	defer client.Close()

	ch, _, err := client.OpenChannel(ReqProvision, nil)
	if err != nil {
		return err
	}
	defer ch.Close()

	buf := &bytes.Buffer{}
	if _, err = io.Copy(buf, ch.Stderr()); err != nil {
		return fmt.Errorf("failed to read key pair from channel: %v", err)
	}
	var keys *PackedKeys
	if err := json.NewDecoder(buf).Decode(&keys); err != nil {
		return err
	}
	return writeKeys(fqdn, dataDir, keys.Key, keys.Cert)
}

func readToken(token string) (string, error) {
	if !strings.HasPrefix(token, "/") {
		return token, nil
	}
	// treat it as a file
	out, err := ioutil.ReadFile(token)
	if err != nil {
		return "", nil
	}
	return string(out), nil
}

type PackedKeys struct {
	Key  []byte `json:"key"`
	Cert []byte `json:"cert"`
}
