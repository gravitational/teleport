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

	"github.com/gravitational/session"
	"github.com/gravitational/trace"
	"github.com/mailgun/lemma/secret"
)

func (cmd *Command) GenerateToken(domainName, role string, ttl time.Duration,
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
		token, err = cmd.client.GenerateToken(domainName, role, ttl)
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
