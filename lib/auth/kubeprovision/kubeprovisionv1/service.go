/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package kubeprovisionv1

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubeprovision/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

// Backend interface for manipulating KubeProvision resources.
type Backend interface {
	services.KubeProvisions
}

// KubeProvisionServiceConfig holds configuration options for
// the KubeProvisions gRPC service.
type KubeProvisionServiceConfig struct {
	Authorizer authz.Authorizer
	Backend    Backend
	Logger     logrus.FieldLogger
}

// NewKubeProvisionService returns a new instance of the KubeProvisionService.
func NewKubeProvisionService(cfg KubeProvisionServiceConfig) (*KubeProvisionService, error) {
	if cfg.Authorizer == nil {
		return nil, trace.BadParameter("authorizer is required")
	}
	if cfg.Backend == nil {
		return nil, trace.BadParameter("backend service is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = logrus.WithField(teleport.ComponentKey, "kube_provision")
	}
	return &KubeProvisionService{
		logger:     cfg.Logger,
		authorizer: cfg.Authorizer,
		backend:    cfg.Backend,
	}, nil
}

// KubeProvisionService implements the teleport.kubeprovision.v1.KubeProvisionService RPC service.
type KubeProvisionService struct {
	// UnsafeKubeProvisionServiceServer is embedded to opt out of forward compatibility for this service.
	// Added methods to KubeProvisionServiceServer will result in compilation errors, which is what we want.
	pb.UnsafeKubeProvisionServiceServer

	backend    Backend
	authorizer authz.Authorizer
	logger     logrus.FieldLogger
}

func (k *KubeProvisionService) authorize(ctx context.Context, adminAction bool, verb string, additionalVerbs ...string) error {
	authCtx, err := k.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = authCtx.CheckAccessToKind(types.KindKubeProvision, verb, additionalVerbs...)
	if err != nil {
		return trace.Wrap(err)
	}

	if adminAction {
		err = authCtx.AuthorizeAdminAction()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetKubeProvision gets a KubeProvision by name. It will return an error if the KubeProvision does not exist.
func (k *KubeProvisionService) GetKubeProvision(ctx context.Context, req *pb.GetKubeProvisionRequest) (*pb.KubeProvision, error) {
	err := k.authorize(ctx, false, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Name == "" {
		return nil, trace.BadParameter("name: must be non-empty")
	}

	out, err := k.backend.GetKubeProvision(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// ListKubeProvisions lists all KubeProvisions.
func (k *KubeProvisionService) ListKubeProvisions(
	ctx context.Context, req *pb.ListKubeProvisionsRequest,
) (*pb.ListKubeProvisionsResponse, error) {
	err := k.authorize(ctx, false, types.VerbRead, types.VerbList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, next, err := k.backend.ListKubeProvisions(ctx, int(req.PageSize), req.PageToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pb.ListKubeProvisionsResponse{
		KubeProvisions: out,
		NextPageToken:  next,
	}, nil
}

// CreateKubeProvision creates a new KubeProvision. It will return an error if the KubeProvision already
// exists.
func (k *KubeProvisionService) CreateKubeProvision(
	ctx context.Context, req *pb.CreateKubeProvisionRequest,
) (*pb.KubeProvision, error) {
	err := k.authorize(ctx, true, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = services.ValidateKubeProvision(req.KubeProvision)
	if err != nil {
		return nil, trace.Wrap(err, "validating object")
	}

	out, err := k.backend.CreateKubeProvision(ctx, req.KubeProvision)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil

}

// UpsertKubeProvision creates a new KubeProvision or forcefully updates an existing KubeProvision.
// This is a function rather than a method so that it can be used by the gRPC service
// and the auth server init code when dealing with resources to be applied at startup.
func UpsertKubeProvision(
	ctx context.Context,
	backend Backend,
	object *pb.KubeProvision,
) (*pb.KubeProvision, error) {
	if err := services.ValidateKubeProvision(object); err != nil {
		return nil, trace.Wrap(err, "validating object")
	}
	out, err := backend.UpsertKubeProvision(ctx, object)
	return out, trace.Wrap(err)
}

// UpdateKubeProvision updates an existing KubeProvision. It will throw an error if the KubeProvision does
// not exist.
func (k *KubeProvisionService) UpdateKubeProvision(
	ctx context.Context, req *pb.UpdateKubeProvisionRequest,
) (*pb.KubeProvision, error) {
	err := k.authorize(ctx, true, types.VerbUpdate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = services.ValidateKubeProvision(req.KubeProvision)
	if err != nil {
		return nil, trace.Wrap(err, "validating object")
	}

	object, err := k.backend.UpdateKubeProvision(ctx, req.KubeProvision)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return object, nil
}

// UpsertKubeProvision creates a new KubeProvision or forcefully updates an existing KubeProvision.
func (k *KubeProvisionService) UpsertKubeProvision(ctx context.Context, req *pb.UpsertKubeProvisionRequest) (*pb.KubeProvision, error) {
	err := k.authorize(ctx, true, types.VerbUpdate, types.VerbCreate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	object, err := UpsertKubeProvision(ctx, k.backend, req.GetKubeProvision())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return object, nil
}

// DeleteKubeProvision deletes an existing KubeProvision. It will throw an error if the KubeProvision does
// not exist.
func (k *KubeProvisionService) DeleteKubeProvision(
	ctx context.Context, req *pb.DeleteKubeProvisionRequest,
) (*emptypb.Empty, error) {
	err := k.authorize(ctx, true, types.VerbDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = k.backend.DeleteKubeProvision(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}
