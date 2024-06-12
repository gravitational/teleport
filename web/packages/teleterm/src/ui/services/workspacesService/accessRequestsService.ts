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

import { ResourceKind } from 'shared/components/AccessRequests/NewRequest';

import type { PendingAccessRequest } from '../workspacesService';

export class AccessRequestsService {
  constructor(
    private getState: () => {
      isBarCollapsed: boolean;
      pending: PendingAccessRequest;
    },
    private setState: (
      draftState: (draft: {
        isBarCollapsed: boolean;
        pending: PendingAccessRequest;
      }) => void
    ) => void
  ) {}

  getCollapsed() {
    return this.getState().isBarCollapsed;
  }

  toggleBar() {
    this.setState(draftState => {
      draftState.isBarCollapsed = !draftState.isBarCollapsed;
    });
  }

  getPendingAccessRequest() {
    return this.getState().pending;
  }

  clearPendingAccessRequest() {
    this.setState(draftState => {
      draftState.pending = getEmptyPendingAccessRequest();
    });
  }

  getAddedResourceCount() {
    const pendingAccessRequest = this.getState().pending;
    return (
      Object.keys(pendingAccessRequest.node).length +
      Object.keys(pendingAccessRequest.db).length +
      Object.keys(pendingAccessRequest.app).length +
      Object.keys(pendingAccessRequest.kube_cluster).length +
      Object.keys(pendingAccessRequest.windows_desktop).length +
      Object.keys(pendingAccessRequest.user_group).length
    );
  }

  addOrRemoveResource(
    kind: ResourceKind,
    resourceId: string,
    resourceName: string
  ) {
    this.setState(draftState => {
      const kindIds = draftState.pending[kind];
      if (kindIds[resourceId]) {
        delete kindIds[resourceId];
      } else {
        kindIds[resourceId] = resourceName || resourceId;
      }
    });
  }
}

export function getEmptyPendingAccessRequest() {
  return {
    node: {},
    db: {},
    kube_cluster: {},
    app: {},
    role: {},
    windows_desktop: {},
    user_group: {},
  };
}
