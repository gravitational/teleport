package collections

import (
	"io"
	"time"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/devicetrust"
)

type deviceCollection struct {
	devices []*devicepb.Device
}

func NewDeviceCollection(devices []*devicepb.Device) ResourceCollection {
	return &deviceCollection{devices: devices}
}

func (c *deviceCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.devices))
	for i, dev := range c.devices {
		resources[i] = types.DeviceToResource(dev)
	}
	return resources
}

func (c *deviceCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"ID", "OS Type", "Asset Tag", "Enrollment Status", "Creation Time", "Last Updated"})
	for _, device := range c.devices {
		t.AddRow([]string{
			device.Id,
			devicetrust.FriendlyOSType(device.OsType),
			device.AssetTag,
			devicetrust.FriendlyDeviceEnrollStatus(device.EnrollStatus),
			device.CreateTime.AsTime().Format(time.RFC3339),
			device.UpdateTime.AsTime().Format(time.RFC3339),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
