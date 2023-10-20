/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { AttemptStatus } from 'shared/hooks/useAsync';

import { assertUnreachable } from 'teleterm/ui/utils';

export type EmptyTableStatus =
  | { status: 'processing' }
  | { status: 'error' }
  | { status: 'no-resources'; showEnrollingResourcesHint: boolean };

export function getEmptyTableStatus(
  fetchAttemptStatus: AttemptStatus,
  searchQuery: string | undefined,
  canAddResources: boolean
): EmptyTableStatus {
  switch (fetchAttemptStatus) {
    case 'error':
      return { status: 'error' };
    case '':
      return { status: 'processing' };
    case 'processing':
      return { status: 'processing' };
    case 'success': {
      // We don't want to inform the user about being able to add new resources if they're actively
      // looking for a resource by using search.
      const isSearchQueryEmpty = !searchQuery || searchQuery.trim() === '';
      const showEnrollingResourcesHint = isSearchQueryEmpty && canAddResources;

      return { status: 'no-resources', showEnrollingResourcesHint };
    }
    default: {
      assertUnreachable(fetchAttemptStatus);
    }
  }
}

/**
 *  getEmptyTableText returns text to be used in an async resource table.
 *
 *  @example
 *  // Successfully fetched with zero results returned
 *  getEmptyTableText(fetchAttempt.status, "servers"); // "No servers found"
 *
 *  @param status - EmptyTableStatus from getEmptyTableStatus
 *  @param pluralResourceNoun - String that represents the plural of a resource, i.e. "servers", "databases"
 */
export function getEmptyTableText(
  status: EmptyTableStatus,
  pluralResourceNoun: string
): { emptyText: string; emptyHint: string } {
  switch (status.status) {
    case 'error':
      return {
        emptyText: `Failed to fetch ${pluralResourceNoun}.`,
        emptyHint: undefined,
      };
    case 'processing':
      return {
        emptyText: 'Searching…',
        emptyHint: undefined,
      };
    case 'no-resources': {
      return {
        emptyText: `No ${pluralResourceNoun} found.`,
        // TODO(ravicious): It'd be nice to include a link to Discover. However, all external links
        // opened by the browser need to be allowlisted (see setWindowOpenHandler), so we should
        // allow opening links only to the currently active cluster and its leafs.
        //
        // However, the main process which does allowlisting doesn't know which clusters the user is
        // logged into. It cannot get this info from the renderer because the renderer could be
        // compromised.
        //
        // Instead, the renderer should notify the main process that the cluster list has changed
        // and the main process should get a list of clusters from the tsh daemon.
        //
        // This is time consuming to set up, so for now we're skipping the link.
        emptyHint: status.showEnrollingResourcesHint
          ? 'You can add them in the Teleport Web UI.'
          : undefined,
      };
    }
    default: {
      assertUnreachable(status);
    }
  }
}
