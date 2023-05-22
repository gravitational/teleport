package service

import (
	"context"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// S is the Jamf service implementation.
//
// The Jamf service is an MDM service specialization that syncs device inventory
// from Jamf to the Auth Server/DeviceTrustService.
type S struct {
	logger  log.FieldLogger
	config  *servicecfg.JamfConfig
	devices devicepb.DeviceTrustServiceClient
}

// Opts are creation options from [S].
type Opts struct {
	Logger        log.FieldLogger
	Config        *servicecfg.JamfConfig
	DevicesClient devicepb.DeviceTrustServiceClient
}

// New creates a new [S] instance.
func New(opts Opts) (*S, error) {
	switch {
	case opts.Logger == nil:
		return nil, trace.BadParameter("parameter Logger required")
	case opts.Config == nil:
		return nil, trace.BadParameter("parameter Config required")
	case opts.DevicesClient == nil:
		return nil, trace.BadParameter("parameter DevicesClient required")
	}

	cfg := opts.Config
	if err := types.ValidateJamfSpecV1(cfg.Spec); err != nil {
		return nil, trace.Wrap(err, "invalid Jamf configuration")
	}

	return &S{
		config:  cfg,
		logger:  opts.Logger,
		devices: opts.DevicesClient,
	}, nil
}

// Run starts the Jamf service, blocking until the context is closed or a fatal
// error occurs.
func (s *S) Run(ctx context.Context) error {
	s.logger.Info("Jamf service successfully started")

	// The service logs all known devices (for easy debugging) and blocks until
	// Teleport is stopped.
	// It'll be made to be more useful in future iterations.
	if err := s.listAndExecOnDevices(ctx, func(d *devicepb.Device) {
		s.logger.Debugf("Found device %v = %s/%v", d.Id, d.OsType, d.AssetTag)
	}); err != nil {
		s.logger.WithError(err).Warn("Failed to list devices")
	}

	<-ctx.Done()

	s.logger.Info("Exited")
	return ctx.Err()
}

func (s *S) listAndExecOnDevices(ctx context.Context, f func(d *devicepb.Device)) error {
	var pageToken string
	for {
		resp, err := s.devices.ListDevices(ctx, &devicepb.ListDevicesRequest{
			PageToken: pageToken,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		for _, dev := range resp.Devices {
			f(dev)
		}
		if resp.NextPageToken == "" {
			return nil
		}
		pageToken = resp.NextPageToken
	}
}
