/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package collections

import (
	"io"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
)

type vnetConfigCollection struct {
	vnetConfig *vnet.VnetConfig
}

func NewVnetConfigCollection(vnetConfig *vnet.VnetConfig) *vnetConfigCollection {
	return &vnetConfigCollection{vnetConfig: vnetConfig}
}

func (c *vnetConfigCollection) Resources() []types.Resource {
	return []types.Resource{types.Resource153ToLegacy(c.vnetConfig)}
}

func (c *vnetConfigCollection) WriteText(w io.Writer, verbose bool) error {
	var dnsZoneSuffixes []string
	for _, dnsZone := range c.vnetConfig.Spec.CustomDnsZones {
		dnsZoneSuffixes = append(dnsZoneSuffixes, dnsZone.Suffix)
	}
	t := asciitable.MakeTable([]string{"IPv4 CIDR range", "Custom DNS Zones"})
	t.AddRow([]string{
		c.vnetConfig.GetSpec().GetIpv4CidrRange(),
		strings.Join(dnsZoneSuffixes, ", "),
	})
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
