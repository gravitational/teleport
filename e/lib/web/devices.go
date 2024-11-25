package web

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) listDevicesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var devices []*devicepb.Device
	var nextPageToken string
	switch assetTag := r.URL.Query().Get("search"); {
	case assetTag != "":
		resp, err := clt.DevicesClient().FindDevices(r.Context(), &devicepb.FindDevicesRequest{
			IdOrTag: assetTag,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		devices = resp.Devices
	default:
		listReq, err := valuesToProtoListDevicesRequest(r.URL.Query())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp, err := clt.DevicesClient().ListDevices(r.Context(), listReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		devices = resp.Devices
		nextPageToken = resp.NextPageToken
	}

	return &ui.ListDevicesResponse{
		Items:    toUIDevices(devices),
		StartKey: nextPageToken,
	}, nil
}

func (p *Plugin) listDevicesByUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	// most defaults in the web UI are 30 for tables
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	query := r.URL.Query()
	// limit value is expected to be non empty and convertible to int.
	limit := query.Get("limit")

	var pageSize int32 // Use server default if not provided.
	if limit != "" {
		parsedLimit, err := strconv.ParseInt(limit, 10, 32)
		if err != nil {
			return nil, trace.BadParameter("failed to parse limit: %v", query.Get("limit"))
		}
		pageSize = int32(parsedLimit)
	}

	listReq := &devicepb.ListDevicesByUserRequest{
		PageSize:  pageSize,
		PageToken: query.Get("startKey"),
	}

	resp, err := clt.DevicesClient().ListDevicesByUser(r.Context(), listReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.ListDevicesResponse{
		Items:    toUIDevices(resp.Devices),
		StartKey: resp.NextPageToken,
	}, nil
}

// valuesToProtoListDevicesRequest builds devicepb.ListDevicesRequest from http request url.Values
func valuesToProtoListDevicesRequest(query url.Values) (*devicepb.ListDevicesRequest, error) {
	// limit value is expected to be non empty and convertible to int.
	pageSize, err := strconv.ParseInt(query.Get("limit"), 10, 32)
	if err != nil {
		return nil, trace.BadParameter("failed to parse limit: %v", query.Get("limit"))
	}

	// Backend handles zeroed or negative page sizes.
	return &devicepb.ListDevicesRequest{
		View:      devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		PageSize:  int32(pageSize),
		PageToken: query.Get("startKey"),
	}, nil
}

// copy only those fields required for web ui.
func toUIDevices(devices []*devicepb.Device) []ui.Device {
	uiDevices := make([]ui.Device, 0, len(devices))
	for _, v := range devices {
		uiDevices = append(uiDevices,
			ui.Device{
				ID:           v.Id,
				AssetTag:     v.AssetTag,
				OSType:       devicetrust.FriendlyOSType(v.OsType),
				EnrollStatus: devicetrust.FriendlyDeviceEnrollStatus(v.EnrollStatus),
				Owner:        v.Owner,
				CreateTime:   v.CreateTime.AsTime(),
			},
		)
	}

	return uiDevices
}
