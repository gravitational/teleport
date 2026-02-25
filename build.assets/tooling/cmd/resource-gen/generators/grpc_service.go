/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package generators

import (
	"fmt"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type grpcServiceData struct {
	resourceBase
	ProtoAlias string // e.g. "gizmov1pb" — pb-suffixed alias for proto imports in gRPC package
	Audit      spec.AuditConfig
	Hooks      spec.HooksConfig
}

// GenerateGRPCService renders a gRPC service implementation.
func GenerateGRPCService(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	base := newResourceBase(rs, module)
	d := grpcServiceData{
		resourceBase: base,
		ProtoAlias:   base.PkgAlias + "pb",
		Audit:        rs.Audit,
		Hooks:        rs.Hooks,
	}
	d.QualType = fmt.Sprintf("*%spb.%s", base.PkgAlias, base.Kind)
	return render("grpc-service", grpcServiceTmpl, d)
}

var grpcServiceTmpl = mustReadTemplate("grpc_service.go.tmpl")

// GenerateGRPCServiceCustom renders the scaffold file (service.go)
// with validation and audit event stubs that the developer fills in.
func GenerateGRPCServiceCustom(rs spec.ResourceSpec, module string) (string, error) {
	if err := rs.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	base := newResourceBase(rs, module)
	d := grpcServiceData{
		resourceBase: base,
		ProtoAlias:   base.PkgAlias + "pb",
		Audit:        rs.Audit,
		Hooks:        rs.Hooks,
	}
	d.QualType = fmt.Sprintf("*%spb.%s", base.PkgAlias, base.Kind)
	return render("grpc-service-custom", grpcServiceCustomTmpl, d)
}

var grpcServiceCustomTmpl = mustReadTemplate("grpc_service_scaffold.go.tmpl")

