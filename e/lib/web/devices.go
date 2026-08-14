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

func (p *Plugin) listDevicesHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var devices []*devicepb.Device
	var nextPageToken string
	switch assetTag := r.URL.Query().Get("search"); {
	case assetTag != "":
		resp, err := clt.DevicesClient().FindDevices(r.Context(), devicepb.FindDevicesRequest_builder{
			IdOrTag: assetTag,
		}.Build())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		devices = resp.GetDevices()
	default:
		listReq, err := valuesToProtoListDevicesRequest(r.URL.Query())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resp, err := clt.DevicesClient().ListDevices(r.Context(), listReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		devices = resp.GetDevices()
		nextPageToken = resp.GetNextPageToken()
	}

	return &ui.ListDevicesResponse{
		Items:    toUIDevices(devices),
		StartKey: nextPageToken,
	}, nil
}

func (p *Plugin) listDevicesByUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (any, error) {
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

	listReq := devicepb.ListDevicesByUserRequest_builder{
		PageSize:  pageSize,
		PageToken: query.Get("startKey"),
	}.Build()

	resp, err := clt.DevicesClient().ListDevicesByUser(r.Context(), listReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.ListDevicesResponse{
		Items:    toUIDevices(resp.GetDevices()),
		StartKey: resp.GetNextPageToken(),
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
	return devicepb.ListDevicesRequest_builder{
		View:      devicepb.DeviceView_DEVICE_VIEW_LIST,
		PageSize:  int32(pageSize),
		PageToken: query.Get("startKey"),
	}.Build(), nil
}

// copy only those fields required for web ui.
func toUIDevices(devices []*devicepb.Device) []ui.Device {
	uiDevices := make([]ui.Device, 0, len(devices))
	for _, v := range devices {
		var source *ui.DeviceSource
		if v.HasSource() {
			source = &ui.DeviceSource{
				Name:   v.GetSource().GetName(),
				Origin: v.GetSource().GetOrigin(),
			}
		}

		uiDevices = append(uiDevices,
			ui.Device{
				ID:           v.GetId(),
				AssetTag:     v.GetAssetTag(),
				OSType:       devicetrust.FriendlyOSType(v.GetOsType()),
				EnrollStatus: devicetrust.FriendlyDeviceEnrollStatus(v.GetEnrollStatus()),
				Owner:        v.GetOwner(),
				CreateTime:   v.GetCreateTime().AsTime(),
				Source:       source,
			},
		)
	}

	return uiDevices
}
