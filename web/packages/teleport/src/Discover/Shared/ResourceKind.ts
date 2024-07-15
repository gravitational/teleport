/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import type { JoinRole } from 'teleport/services/joinToken';

export enum ResourceKind {
  Application,
  Database,
  Desktop,
  Kubernetes,
  Server,
  SamlApplication,
  Discovery,
  ConnectMyComputer,
}

export function resourceKindToJoinRole(kind: ResourceKind): JoinRole {
  switch (kind) {
    case ResourceKind.Application:
      return 'App';
    case ResourceKind.Database:
      return 'Db';
    case ResourceKind.Desktop:
      return 'WindowsDesktop';
    case ResourceKind.Kubernetes:
      return 'Kube';
    case ResourceKind.Server:
      return 'Node';
    case ResourceKind.Discovery:
      return 'Discovery';
    case ResourceKind.ConnectMyComputer:
      return 'Node';
  }
}

export function resourceKindToPreferredResource(kind: ResourceKind): Resource {
  switch (kind) {
    case ResourceKind.Application:
      return Resource.WEB_APPLICATIONS;
    case ResourceKind.Database:
      return Resource.DATABASES;
    case ResourceKind.Desktop:
      return Resource.WINDOWS_DESKTOPS;
    case ResourceKind.Kubernetes:
      return Resource.KUBERNETES;
    case ResourceKind.Server:
      return Resource.SERVER_SSH;
    case ResourceKind.Discovery:
      return Resource.UNSPECIFIED;
    case ResourceKind.ConnectMyComputer:
      return Resource.SERVER_SSH;
  }
}
