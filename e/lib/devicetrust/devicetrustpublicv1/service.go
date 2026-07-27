// Package devicetrustpublicv1 implements the auth-side handler for
// teleport.devicetrust.public.v1.DeviceTrustService.
//
// The public Device Trust service is the unauthenticated counterpart to
// teleport.devicetrust.v1.DeviceTrustService. It exists so that mobile devices
// can carry out enrollment and device auth without first having to obtain a
// user cert through a full login procedure.
//
// See RFD 32e for more details.
package devicetrustpublicv1

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/services"
)

// Service implements the
// teleport.devicetrust.public.v1.DeviceTrustService RPC service.
type Service struct {
	devicetrustpublicv1pb.UnimplementedDeviceTrustServiceServer

	logger  *slog.Logger
	storage *storage.S
}

// ServiceParams holds creation parameters for [Service].
type ServiceParams struct {
	Logger        *slog.Logger
	EnrollPairing services.EnrollPairing
	Storage       *storage.S
}

// New creates a new public Device Trust [Service].
func New(params ServiceParams) (*Service, error) {
	if params.EnrollPairing == nil {
		return nil, trace.BadParameter("parameter EnrollPairing required")
	}
	if params.Storage == nil {
		return nil, trace.BadParameter("parameter Storage required")
	}
	logger := params.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		logger:  logger.With(teleport.ComponentKey, "devicetrust.public.service"),
		storage: params.Storage,
	}, nil
}

// CreatePairedDeviceEnrollToken creates a device enrollment token given an
// enroll pairing token and collected device data.
func (s *Service) CreatePairedDeviceEnrollToken(
	ctx context.Context,
	req *devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenRequest,
) (*devicetrustpublicv1pb.CreatePairedDeviceEnrollTokenResponse, error) {
	if req.GetEnrollPairingToken() == "" {
		return nil, trace.BadParameter("enroll_pairing_token required")
	}
	if req.GetDeviceData() == nil || proto.Equal(req.GetDeviceData(), &devicepb.DeviceCollectedData{}) {
		return nil, trace.BadParameter("device_data required")
	}

	return nil, trace.NotImplemented("method CreatePairedDeviceEnrollToken not implemented")
}
