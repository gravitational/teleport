package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

func Register(fqdn, dataDir, token string, servers []utils.NetAddr) error {
	method, err := NewTokenAuth(fqdn, token)
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

type PackedKeys struct {
	Key  []byte `json:"key"`
	Cert []byte `json:"cert"`
}
