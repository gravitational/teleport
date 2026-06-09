package accessgraph

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/constants"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accessgraph"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/devicetrust/assertserver"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/modules"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

var (
	errDeviceTrustDisabled = &trace.BadParameterError{
		Message: "device trust disabled by cluster settings",
	}
	authnDisabledLogOnce sync.Once
)

// ReportAuthorizedKeys handler is used by nodes to report their authorized keys.
// Although the handler is bi-directional, currently it only works in client-to-server
// mode, where the client sends a stream of authorized keys to the server.
// Future implementations might extend this to work in server-to-client mode as well
// where the server sends commands to the client.
func (s *Service) ReportAuthorizedKeys(in accessgraphsecretsv1pb.SecretsScannerService_ReportAuthorizedKeysServer) error {
	ctx := in.Context()
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleNode)) {
		return trace.AccessDenied("only nodes are allowed to report authorized keys")
	}

	hostID, err := auth.ExtractHostID(authCtx.Identity.GetIdentity().Username, s.clusterName)
	if err != nil {
		return trace.Wrap(err, "failed to extract host ID from the user name")
	}
	var accumKeys []*accessgraphsecretsv1pb.AuthorizedKey
	for {
		req, err := in.Recv()
		if err != nil {
			if len(accumKeys) > 0 {
				s.log.WarnContext(ctx, "Authorized keys stream ended before sync, deleting all keys", "error", err)
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err)
		}

		if req == nil {
			return trace.BadParameter("request is nil")
		}

		for _, key := range req.GetKeys() {
			if err := accessgraph.ValidateAuthorizedKey(key); err != nil {
				return trace.Wrap(err, "failed to validate authorized key")
			}
			if key.GetSpec().GetHostId() != hostID {
				return trace.BadParameter("host ID mismatch")
			}
		}

		switch req.GetOperation() {
		case accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_ADD:
			accumKeys = append(accumKeys, req.GetKeys()...)
		case accessgraphsecretsv1pb.OperationType_OPERATION_TYPE_SYNC:
			var errs []error
			if err := s.deleteRemovedAuthorizedKeys(ctx, hostID, accumKeys); err != nil {
				errs = append(errs, err)
			}

			for _, key := range accumKeys {
				if err := s.upsertAuthorizedKey(ctx, key); err != nil {
					errs = append(errs, err)
				}
			}

			s.usageReporter.AnonymizeAndSubmit(
				&usagereporter.AccessGraphSecretsScanAuthorizedKeysEvent{
					HostId:    hostID,
					TotalKeys: uint64(len(accumKeys)),
				})

			if len(errs) > 0 {
				return trace.NewAggregate(errs...)
			}
			accumKeys = nil
		default:
			return trace.BadParameter("unsupported operation type: %v", req.GetOperation())
		}
	}

}

// ReportSecrets handler is used by trusted devices to report secrets found on the host.
func (s *Service) ReportSecrets(in accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer) error {
	// Is device authentication allowed?
	authPref, err := s.authPreferenceGetter(in.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := isDeviceAuthnAllowed(authPref.GetDeviceTrust(), s.modules); err != nil {
		authnDisabledLogOnce.Do(func() {
			s.log.WarnContext(in.Context(), "Device authentication attempted, but device trust is disabled by cluster settings")
		})
		return trace.Wrap(err)
	}

	ceremony, err := s.deviceAssertionServer()
	if err != nil {
		return trace.Wrap(err, "failed to get device assertion server")
	}
	dev, err := ceremony.AssertDevice(in.Context(), streamAdapter{in})
	if err != nil {
		return trace.Wrap(err)
	}
	devID := dev.GetId()

	var allKeys []*accessgraphsecretsv1pb.PrivateKey
	for {
		req, err := in.Recv()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return trace.Wrap(err)
		}

		keys := req.GetPrivateKeys()
		if keys == nil {
			return trace.BadParameter("expected private keys in the request after device authentication, got nil")
		}
		for _, key := range keys.GetKeys() {
			if err := accessgraph.ValidatePrivateKey(key); err != nil {
				return trace.Wrap(err, "failed to validate private key")
			}

			// replace key with the system deviceID
			key.GetSpec().SetDeviceId(devID)
			allKeys = append(allKeys, key)
		}
	}

	if err := s.deleteRemovedPrivateKeys(in.Context(), devID, allKeys); err != nil {
		return trace.Wrap(err)
	}

	for _, key := range allKeys {
		if err := s.upsertPrivateKey(in.Context(), key); err != nil {
			return trace.Wrap(err)
		}
	}

	s.usageReporter.AnonymizeAndSubmit(
		&usagereporter.AccessGraphSecretsScanSSHPrivateKeysEvent{
			DeviceId:     devID,
			TotalKeys:    uint64(len(allKeys)),
			DeviceOsType: dev.GetOsType().String(),
		})

	return nil
}

func isDeviceAuthnAllowed(dt *types.DeviceTrust, m modules.Modules) error {
	if dtconfig.GetEffectiveMode(dt, m) == constants.DeviceTrustModeOff {
		return trace.Wrap(errDeviceTrustDisabled)
	}
	return nil
}

// deleteRemovedAuthorizedKeys deletes keys that are not present in the newKeys list
// but are present in the backend service.
func (s *Service) deleteRemovedAuthorizedKeys(ctx context.Context, hostID string, newKeys []*accessgraphsecretsv1pb.AuthorizedKey) error {
	oldKeys, err := s.listAllAuthorizedKeysForServer(ctx, hostID)
	if err != nil {
		return trace.Wrap(err, "failed to list all authorized keys for server")
	}

	var errs []error

	newKeysMap := make(map[string]struct{})
	for _, key := range newKeys {
		newKeysMap[key.GetMetadata().GetName()] = struct{}{}
	}

	for _, key := range oldKeys {
		name := key.GetMetadata().GetName()
		if _, ok := newKeysMap[name]; !ok {
			if err := s.deleteAuthorizedKey(ctx, key.GetSpec().GetHostId(), name); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return trace.NewAggregate(errs...)
}

// listAllAuthorizedKeysForServer lists all authorized keys for a given host.
func (s *Service) listAllAuthorizedKeysForServer(ctx context.Context, hostID string) ([]*accessgraphsecretsv1pb.AuthorizedKey, error) {
	var (
		pageToken = ""
		allKeys   []*accessgraphsecretsv1pb.AuthorizedKey
	)
	for {
		keys, nextKey, err := s.secretsService.ListAuthorizedKeysForServer(ctx, hostID, 0 /* use the default value */, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allKeys = append(allKeys, keys...)
		if nextKey == "" {
			break
		}
		pageToken = nextKey

	}
	return allKeys, nil
}

// upsertAuthorizedKey upserts a new authorized key.
func (s *Service) upsertAuthorizedKey(ctx context.Context, key *accessgraphsecretsv1pb.AuthorizedKey) error {
	if err := accessgraph.ValidateAuthorizedKey(key); err != nil {
		return trace.Wrap(err, "failed to validate authorized key")
	}

	_, err := s.secretsService.UpsertAuthorizedKey(ctx, key)
	if err != nil {
		return trace.Wrap(err, "failed to upsert authorized key")
	}

	return nil
}

// deleteAuthorizedKey deletes a specific authorized key.
func (s *Service) deleteAuthorizedKey(ctx context.Context, hostID, name string) error {
	if err := s.secretsService.DeleteAuthorizedKey(ctx, hostID, name); err != nil {
		return trace.Wrap(err, "failed to delete authorized key")
	}
	return nil
}

// listAllPrivateKeysForDevice lists all authorized keys for a given deviceID.
func (s *Service) listAllPrivateKeysForDevice(ctx context.Context, deviceID string) ([]*accessgraphsecretsv1pb.PrivateKey, error) {
	var (
		pageToken = ""
		allKeys   []*accessgraphsecretsv1pb.PrivateKey
	)
	for {
		keys, nextKey, err := s.secretsService.ListPrivateKeysForDevice(ctx, deviceID, 0 /* use the default value */, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		allKeys = append(allKeys, keys...)
		if nextKey == "" {
			break
		}
		pageToken = nextKey

	}
	return allKeys, nil
}

// upsertPrivateKey upserts a new private key.
func (s *Service) upsertPrivateKey(ctx context.Context, key *accessgraphsecretsv1pb.PrivateKey) error {
	if err := accessgraph.ValidatePrivateKey(key); err != nil {
		return trace.Wrap(err, "failed to validate private key")
	}

	_, err := s.secretsService.UpsertPrivateKey(ctx, key)

	return trace.Wrap(err, "failed to upsert private key")

}

// deletePrivateKey deletes a specific private key.
func (s *Service) deletePrivateKey(ctx context.Context, deviceID, name string) error {
	err := s.secretsService.DeletePrivateKey(ctx, deviceID, name)
	return trace.Wrap(err, "failed to delete private key")
}

// deleteRemovedAuthorizedKeys deletes keys that are not present in the newKeys list
// but are present in the backend service.
func (s *Service) deleteRemovedPrivateKeys(ctx context.Context, deviceID string, newKeys []*accessgraphsecretsv1pb.PrivateKey) error {
	oldKeys, err := s.listAllPrivateKeysForDevice(ctx, deviceID)
	if err != nil {
		return trace.Wrap(err, "failed to list all authorized keys for server")
	}

	newKeysMap := make(map[string]struct{})
	for _, key := range newKeys {
		newKeysMap[key.GetMetadata().GetName()] = struct{}{}
	}

	for _, key := range oldKeys {
		name := key.GetMetadata().GetName()
		if _, ok := newKeysMap[name]; !ok {
			if err := s.deletePrivateKey(ctx, deviceID, name); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

var (
	_ assertserver.AssertDeviceServerStream = streamAdapter{}
)

// streamAdapter is a helper struct that adapts the [accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer]
// stream to the device trust assertion stream [assertserver.AssertDeviceServerStream].
// This is needed because we need to extract the [*devicepb.AssertDeviceRequest] from the stream
// and return the [*devicepb.AssertDeviceResponse] to the stream.
type streamAdapter struct {
	stream accessgraphsecretsv1pb.SecretsScannerService_ReportSecretsServer
}

func (s streamAdapter) Send(rsp *devicepb.AssertDeviceResponse) error {
	msg := accessgraphsecretsv1pb.ReportSecretsResponse_builder{
		DeviceAssertion: proto.ValueOrDefault(rsp),
	}.Build()
	err := s.stream.Send(msg)
	return trace.Wrap(err)
}

func (s streamAdapter) Recv() (*devicepb.AssertDeviceRequest, error) {
	msg, err := s.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if msg.GetDeviceAssertion() == nil {
		return nil, trace.BadParameter("unexpected assert request payload: %T", msg.GetPayload())
	}

	return msg.GetDeviceAssertion(), nil
}
