package collections

import (
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"io"
	"strings"
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
