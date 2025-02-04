/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Acl } from 'teleport/services/user';

import { ResourceKind } from '../../Shared';
import { ResourceSpec } from '../types';

function checkHasAccess(acl: Acl, resourceKind: ResourceKind) {
  const basePerm = acl.tokens.create;
  if (!basePerm) {
    return false;
  }

  switch (resourceKind) {
    case ResourceKind.Application:
      return acl.appServers.read && acl.appServers.list;
    case ResourceKind.Database:
      return acl.dbServers.read && acl.dbServers.list;
    case ResourceKind.Desktop:
      return acl.desktops.read && acl.desktops.list;
    case ResourceKind.Kubernetes:
      return acl.kubeServers.read && acl.kubeServers.list;
    case ResourceKind.Server:
      return acl.nodes.list;
    case ResourceKind.SamlApplication:
      return acl.samlIdpServiceProvider.create;
    case ResourceKind.ConnectMyComputer:
      // This is probably already true since without this permission the user wouldn't be able to
      // add any other resource, but let's just leave it for completeness sake.
      return acl.tokens.create;
    default:
      return false;
  }
}

export function addHasAccessField(
  acl: Acl,
  resources: ResourceSpec[]
): ResourceSpec[] {
  return resources.map(r => {
    const hasAccess = checkHasAccess(acl, r.kind);
    switch (r.kind) {
      case ResourceKind.Database:
        return { ...r, dbMeta: { ...r.dbMeta }, hasAccess };
      default:
        return { ...r, hasAccess };
    }
  });
}
