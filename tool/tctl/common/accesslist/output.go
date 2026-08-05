// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package accesslist

import (
	"fmt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/accesslists/preset"
	"github.com/gravitational/teleport/lib/accesslists/terraform"
	"github.com/gravitational/trace"
)

func (c *Command) writeOutput(al *accesslist.AccessList, accessRoles []*types.RoleV6, members []*accesslist.AccessListMember, alPresetType string) error {
	switch c.output {
	case outputTerraform:
		return trace.Wrap(c.writeOutputTerraform(al, accessRoles, members, alPresetType))
	default:
		return trace.BadParameter("unsupported output %q", c.output)
	}
}

func (c *Command) writeOutputTerraform(al *accesslist.AccessList, accessRoles []*types.RoleV6, members []*accesslist.AccessListMember, alPresetType string) error {
	var providerBlock *terraform.ProviderBlock
	if c.cfg != nil {
		addrs := c.cfg.AuthServerAddresses()
		if len(addrs) > 0 {
			providerBlock = &terraform.ProviderBlock{
				TeleportVersion: teleport.SemVer().Major,
				ProxyAddr:       addrs[0].String(),
			}
		}
	}

	tfConfig := ""
	var err error
	if alPresetType == "" {
		tfConfig, err = terraform.GenerateConfig(al, members, providerBlock)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		roles := make([]terraform.AccessRole, 0, len(accessRoles))
		for _, role := range accessRoles {
			roles = append(roles, terraform.AccessRole{Role: role})
		}
		tfConfig, err = terraform.GenerateConfigWithPresetBuilder(terraform.ConfigParams{
			PresetType:    preset.PresetType(alPresetType),
			AccessListID:  al.GetName(),
			AccessList:    al,
			AccessRoles:   roles,
			Members:       members,
			ProviderBlock: providerBlock,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	fmt.Fprint(c.Stdout, tfConfig)
	return nil
}
