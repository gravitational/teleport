package command

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
)

func (cmd *Command) GenerateToken(fqdn string, ttl time.Duration,
	output, secretKey string) error {

	var token string
	// if secretKey is passed, we use it to initialize the
	// secret service and generate the token locally instead
	// of talking to remote service. this allows us to do offline
	// configuration without running teleport server
	if secretKey != "" {
		key, err := secret.EncodedStringToKey(secretKey)
		if err != nil {
			return trace.Wrap(err)
		}
		secretService, err := secret.New(&secret.Config{KeyBytes: key})
		if err != nil {
			return trace.Wrap(err)
		}

		p, err := session.NewID(secretService)
		if err != nil {
			return trace.Wrap(err)
		}
		token = string(p.SID)
	} else {
		var err error
		token, err = cmd.client.GenerateToken(fqdn, ttl)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if output == "" {
		fmt.Fprintf(cmd.out, token)
	} else {
		if err := ioutil.WriteFile(output, []byte(token), 0644); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
