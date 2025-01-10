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
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	samlidpv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// subcommandRunner is used to create pluggable subcommand under
// idp command. E.g:
// $ tctl idp saml <command> [<args> ...]
// $ tctl idp oidc <command> [<args> ...]
type subcommandRunner interface {
	initialize(parent *kingpin.CmdClause, cfg *servicecfg.Config)
	tryRun(ctx context.Context, selectedCommand string, clientFunc commonclient.InitFunc) (match bool, err error)
}

// IdPCommand implements all commands under "tctl idp".
type IdPCommand struct {
	subcommandRunners []subcommandRunner
}

// samlIdPCommand implements all commands under "tctl idp saml"
type samlIdPCommand struct {
	cmd *kingpin.CmdClause

	testAttributeMapping testAttributeMapping
}

// Initialize installs the base "idp" command and all subcommands.
func (t *IdPCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, cfg *servicecfg.Config) {
	idp := app.Command("idp", "Teleport Identity Provider")

	idp.Alias(`
Examples:
  > tctl idp saml
`)

	t.subcommandRunners = []subcommandRunner{
		&samlIdPCommand{},
	}

	for _, subcommandRunner := range t.subcommandRunners {
		subcommandRunner.initialize(idp, cfg)
	}
}

// TryRun calls tryRun for each subcommand, and returns (false, nil) if none of them match.
func (i *IdPCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	for _, subcommandRunner := range i.subcommandRunners {
		match, err = subcommandRunner.tryRun(ctx, cmd, clientFunc)
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
	samlcmd := parent.Command("saml", "SAML Identity Provider")
	samlcmd.Alias(`
Examples:

  Test attribute mapping with given user and service provider.
  > tctl idp saml test-attribute-mapping --users user.yaml (user spec file or username) --sp sp.yaml (service provider spec file) --format (json,yaml)
`)
	s.cmd = samlcmd

	testAttrMap := samlcmd.Command("test-attribute-mapping", "Test expression evaluation of attribute mapping.")
	testAttrMap.Flag("users", "username or name of a file containing user spec").Required().Short('u').StringsVar(&s.testAttributeMapping.users)
	testAttrMap.Flag("sp", "name of a file containing service provider spec").Required().StringVar(&s.testAttributeMapping.serviceProvider)
	testAttrMap.Flag("format", "output format, 'yaml' or 'json'").StringVar(&s.testAttributeMapping.outFormat)
	testAttrMap.Alias(`
Examples:

	# test with username and service provider file
	> tctl idp saml test-attribute-mapping --users user1 --sp sp.yaml

	# test with multiple usernames and service provider file
	> tctl idp saml test-attribute-mapping --users user1,user2 --sp sp.yaml

	# test with user and service provider file and print output in yaml format
	> tctl idp saml test-attribute-mapping --users user1.yaml --sp sp.yaml --format yaml
`)
	s.testAttributeMapping.cmd = testAttrMap
}

func (s *samlIdPCommand) tryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	switch cmd {
	case s.testAttributeMapping.cmd.FullCommand():
		client, closeFn, err := clientFunc(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}
		defer closeFn(ctx)
		return true, trace.Wrap(s.testAttributeMapping.run(ctx, client))
	default:
		return false, nil
	}
}

// testCommand implements the "tctl idp saml test-attribute-mapping" command.
type testAttributeMapping struct {
	cmd *kingpin.CmdClause

	serviceProvider string
	users           []string
	outFormat       string
}

func (t *testAttributeMapping) run(ctx context.Context, c *authclient.Client) error {
	serviceProvider, err := parseSPFile(t.serviceProvider)
	if err != nil {
		return trace.Wrap(err)
	}

	users, err := getUsersFromAPIOrFile(ctx, t.users, c)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(users) == 0 {
		return trace.BadParameter("users not found in file: %s", t.users)
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

	switch t.outFormat {
	case teleport.YAML:
		if err := utils.WriteYAML(os.Stdout, resp.MappedAttributes); err != nil {
			return trace.Wrap(err)
		}
	case teleport.JSON:
		if err := utils.WriteJSON(os.Stdout, resp.MappedAttributes); err != nil {
			return trace.Wrap(err)
		}
	default:
		for i, mappedAttribute := range resp.MappedAttributes {
			table := asciitable.MakeTable([]string{"Attribute Name", "Attribute Value"})
			if i > 0 {
				fmt.Println("---")
			}
			fmt.Printf("User: %s\n", mappedAttribute.Username)
			for name, value := range mappedAttribute.MappedValues {
				table.AddRow([]string{
					name,
					strings.Join(value.Values, ", "),
				})
			}
			fmt.Println(table.AsBuffer().String())
		}
	}

	return nil
}

// parseSPFile parses service provider spec from given file.
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
	if err := decoder.Decode(&u); err != nil {
		if errors.Is(err, io.EOF) {
			return u, trace.BadParameter("service provider not found in file: %s", fileName)
		}
		return u, trace.Wrap(err)
	}
	return u, nil
}

// getUsersFromAPIOrFile parses user from spec file. If file is not found, it fetches user from backend.
func getUsersFromAPIOrFile(ctx context.Context, usernamesOrFileNames []string, c *authclient.Client) ([]*types.UserV2, error) {
	flattenedUsernamesOrFileNames := flattenSlice(usernamesOrFileNames)
	var users []*types.UserV2

	for _, name := range flattenedUsernamesOrFileNames {
		if _, err := os.Stat(name); os.IsNotExist(err) {
			user, err := c.GetUser(ctx, name, false)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			userV2, ok := user.(*types.UserV2)
			if !ok {
				return nil, trace.BadParameter("unsupported user type %T", user)
			}
			users = append(users, userV2)
		} else {
			f, err := os.Open(name)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			defer f.Close()

			usersFromFile, err := parseUserFromFile(f)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			users = append(users, usersFromFile...)
		}
	}

	return users, nil
}

func parseUserFromFile(r io.Reader) ([]*types.UserV2, error) {
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
