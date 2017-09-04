/*
Copyright 2017 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Package main contains the enterprise edition of tctl CLI tool.
*/

package main

import (
	"fmt"
	"strings"

	"github.com/buger/goterm"

	"github.com/gravitational/teleport/e/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common"

	"github.com/gravitational/trace"
)

func main() {
	commands := []common.CLICommand{
		&common.UserCommand{Impl: &common.UserCommandImpl{
			List: listUsers,
		}},
		&common.NodeCommand{},
		&common.TokenCommand{},
		&common.AuthCommand{},
		&common.ResourceCommand{Impl: &common.ResourceCommandImpl{
			Delete: deleteResource,
			Create: createResource,
		}},
	}
	common.Run(lib.DistroName, commands)
}

func deleteResource(ref services.Ref, client *auth.TunClient) error {
	switch ref.Kind {
	case services.KindRole:
		if err := client.DeleteRole(ref.Name); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("role %s has been deleted\n", ref.Name)
	}
	return nil
}
func createResource(raw *services.UnknownResource, client *auth.TunClient) error {
	switch raw.Kind {
	case services.KindRole:
		role, err := services.GetRoleMarshaler().UnmarshalRole(raw.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		err = role.CheckAndSetDefaults()
		if err != nil {
			return trace.Wrap(err)
		}
		if err := client.UpsertRole(role, backend.Forever); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("created role: %v\n", role.GetName())
	default:
		return trace.BadParameter("creating resources of type %q is not supported", raw.Kind)
	}
	return nil
}

// listUsers performs `tctl users ls` for the enterprise edition. Unlike the OSS
// version, this implementation prints user roles (instead of "allowed logins")
func listUsers(users []services.User, client *auth.TunClient) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintHeader(t, []string{"User", "Roles"})
	for _, u := range users {
		fmt.Fprintf(t, "%v\t%v\n", u.GetName(), strings.Join(u.GetRoles(), ","))
	}
	fmt.Println(t.String())
	return nil
}
