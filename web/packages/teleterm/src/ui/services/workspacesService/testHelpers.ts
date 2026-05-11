/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { rootClusterUri } from 'teleterm/services/tshd/testHelpers';

import { PersistedWorkspace } from '../statePersistence';
import { getEmptyPendingAccessRequest } from './accessRequestsService';
import { makeDocumentCluster } from './documentsService/testHelpers';
import {
  getDefaultUnifiedResourcePreferences,
  Workspace,
} from './workspacesService';

export function makeWorkspace(props?: Partial<Workspace>): Workspace {
  const clusterUri = props?.localClusterUri ?? rootClusterUri;
  const defaultDocument = makeDocumentCluster({ clusterUri });

  return {
    accessRequests: {
      isBarCollapsed: false,
      pending: getEmptyPendingAccessRequest(),
    },
    color: 'purple',
    connectMyComputer: undefined,
    documents: [defaultDocument],
    hasDocumentsToReopen: false,
    localClusterUri: clusterUri,
    location: defaultDocument.uri,
    proxyHost: 'teleport-local.com:3080',
    unifiedResourcePreferences: getDefaultUnifiedResourcePreferences(),
    ...props,
  };
}

export function makePersistedWorkspace(
  props?: Partial<PersistedWorkspace>
): PersistedWorkspace {
  const clusterUri = props?.localClusterUri ?? rootClusterUri;

  return {
    documents: [],
    localClusterUri: clusterUri,
    location: undefined,
    proxyHost: 'teleport-local.com:3080',
    ...props,
  };
}
