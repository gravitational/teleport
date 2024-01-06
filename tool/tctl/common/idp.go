/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	samlidpv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// subCommandRunner is used to create pluggable sub command under
// idp command. E.g:
// $ tctl idp saml <command> [<args> ...]
// $ tctl idp oidc <command> [<args> ...]
type subCommandRunner interface {
	initialize(parent *kingpin.CmdClause, cfg *servicecfg.Config)
	tryRun(ctx context.Context, selectedCommand string, c auth.ClientI) (match bool, err error)
}

// IdPCommand implements all commands under "tctl idp".
type IdPCommand struct {
	subCommandRunners []subCommandRunner
}

// samlIdPCommand implements all commands under "tctl idp saml"
type samlIdPCommand struct {
	cmd *kingpin.CmdClause

	testAttributeMapping testAttributeMapping
}

// Initialize installs the base "idp" command and all sub commands.
func (t *IdPCommand) Initialize(app *kingpin.Application, cfg *servicecfg.Config) {
	idp := app.Command("idp", "Teleport Identity Provider")

	idp.Alias(`
Examples:
  > tctl idp saml
`)

	t.subCommandRunners = []subCommandRunner{
		&samlIdPCommand{},
	}

	for _, subCommandRunner := range t.subCommandRunners {
		subCommandRunner.initialize(idp, cfg)
	}

}

// TryRun calls tryRun for each subc ommand, and if none of them match returns
// (false, nil)
func (i *IdPCommand) TryRun(ctx context.Context, cmd string, c auth.ClientI) (match bool, err error) {
	for _, subCommandRunner := range i.subCommandRunners {
		match, err = subCommandRunner.tryRun(ctx, cmd, c)
		if err != nil {
			return match, trace.Wrap(err)
		}
		if match {
			return match, nil
		}
	}
	return false, nil
}

func (s *samlIdPCommand) initialize(parent *kingpin.CmdClause, cfg *servicecfg.Config) {
	samlcmd := parent.Command("saml", "SAML Identity Provider.")
	samlcmd.Alias(`
Examples:

  Test Attribute Mapping from spconfig.yaml with input traits from user.yaml
  > tctl idp saml test_attribute_mapping --users user.yaml (or username) --serviceprovider sp.yaml (or service provider name)
`)
	s.cmd = samlcmd

	testAttrMap := samlcmd.Command("test_attribute_mapping", "Test the parsing and evaluation of attribute mapping.")
	testAttrMap.Flag("users", "username or name of a file containing user spec").StringsVar(&s.testAttributeMapping.users)
	testAttrMap.Flag("serviceprovider", "service provider name or name of a file containing service provider spec").StringVar(&s.testAttributeMapping.serviceProvider)
	testAttrMap.Alias(`
Examples:

	# test with username and service provider name
	> tctl idp saml test_attribute_mapping --users user1 --serviceprovider mysamlapp

	# test with multiple usernames and service provider name
	> tctl idp saml test_attribute_mapping --users user1,user2 --serviceprovider sp.yaml

	# test with user and service provider file
	> tctl idp saml test_attribute_mapping --users user1.yaml --serviceprovider sp.yaml
`)

	s.testAttributeMapping.cmd = testAttrMap

}

func (s *samlIdPCommand) tryRun(ctx context.Context, cmd string, c auth.ClientI) (match bool, err error) {
	switch cmd {
	case s.testAttributeMapping.cmd.FullCommand():
		return true, trace.Wrap(s.testAttributeMapping.Run(ctx, c))
	default:
		return false, nil
	}
}

// testCommand implements the "tctl idp saml test_attribute_mapping" command.
type testAttributeMapping struct {
	cmd             *kingpin.CmdClause
	serviceProvider string
	users           []string
}

func (t *testAttributeMapping) tryRun(ctx context.Context, cmd string, c auth.ClientI) (match bool, err error) {
	return true, nil
}

func (t *testAttributeMapping) Run(ctx context.Context, c auth.ClientI) error {
	if len(t.serviceProvider) == 0 && len(t.users) == 0 {
		return trace.BadParameter("no attributes to test, --user and --serviceprovider must be set")
	}
	serviceProvider, err := parseSPFile(t.serviceProvider)
	if err != nil {
		return trace.Wrap(err)
	}

	users, err := getUserFromNameOrFile(ctx, t.users, c)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := c.SAMLIdPClient().TestSAMLIdPAttributeMapping(ctx, &samlidpv1.TestSAMLIdPAttributeMappingRequest{
		ServiceProvider: &serviceProvider,
		Users:           users,
	})

	if err != nil {
		if trace.IsNotImplemented(err) {
			return trace.NotImplemented("the server does not support testing SAML attribute mapping")
		}
		return trace.Wrap(err)
	}

	for _, v := range resp.MappedAttributes {
		fmt.Printf("User: %s\n", v.Username)
		for m, v := range v.MappedValues {
			fmt.Printf("%s: %v\n", m, v.Values)
		}
		fmt.Println("--------------")
	}

	return nil
}

// parseSAMLIdP only handles files.
// TODO(sshah): handle service provider name
func parseSPFile(fileName string) (types.SAMLIdPServiceProviderV1, error) {
	var u types.SAMLIdPServiceProviderV1
	var r io.Reader = os.Stdin
	if fileName != "" {
		f, err := os.Open(fileName)
		if err != nil {
			return u, trace.Wrap(err)
		}
		defer f.Close()
		r = f
	}

	decoder := kyaml.NewYAMLOrJSONDecoder(r, defaults.LookaheadBufSize)

	err := decoder.Decode(&u)
	if err != nil {
		return u, trace.Wrap(err)
	}

	return u, nil
}

func getUserFromNameOrFile(ctx context.Context, userfileOrNames []string, c auth.ClientI) ([]*types.UserV2, error) {
	ufromFileOrName := flattenSlice(userfileOrNames)
	var allusers []*types.UserV2

	for _, name := range ufromFileOrName {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			user, err := c.GetUser(ctx, name, false)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			userV2, ok := user.(*types.UserV2)
			if !ok {
				return nil, trace.BadParameter("unsupported user type %T", user)
			}
			allusers = append(allusers, userV2)

		} else {
			var r io.Reader = os.Stdin
			f, err := os.Open(name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			defer f.Close()
			r = f

			users, err := getUserFromFile(r)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			allusers = append(allusers, users...)
		}

	}

	return allusers, nil
}

func getUserFromFile(r io.Reader) ([]*types.UserV2, error) {
	var users []*types.UserV2
	decoder := kyaml.NewYAMLOrJSONDecoder(r, defaults.LookaheadBufSize)
	for {
		var u *types.UserV2
		err := decoder.Decode(&u)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return users, nil
			}
			return nil, trace.Wrap(err)
		}

		users = append(users, u)
	}
}
