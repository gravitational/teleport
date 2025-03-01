/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
  useMemo,
} from 'react';

import { AccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import { Attempt, useAsync } from 'shared/hooks/useAsync';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { getAssumedRequests } from 'teleterm/ui/services/clusters';
import { RootClusterUri } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

export interface AccessRequestsContext {
  /** Determines whether the user can use the Access Requests UI.
   * True when the cluster enables it.
   */
  canUse: boolean;
  fetchRequestsAttempt: Attempt<AccessRequest[]>;
  fetchRequests(): Promise<[AccessRequest[], Error]>;
  /** Maps access request ID to the corresponding request object. */
  assumed: Map<string, AccessRequest>;
}

const AccessRequestsContext = createContext<AccessRequestsContext>(null);

export const AccessRequestsContextProvider: FC<
  PropsWithChildren<{
    rootClusterUri: RootClusterUri;
  }>
> = ({ rootClusterUri, children }) => {
  const appContext = useAppContext();
  const { tshd } = appContext;
  const rootCluster = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters.get(rootClusterUri), [rootClusterUri])
  );

  const assumedObject = useStoreSelector(
    'clustersService',
    useCallback(
      state => getAssumedRequests(state, rootClusterUri),
      [rootClusterUri]
    )
  );
  const assumed = useMemo(
    () => new Map(Object.entries(assumedObject)),
    [assumedObject]
  );

  const canUse = !!rootCluster?.features?.advancedAccessWorkflows;

  const [fetchRequestsAttempt, fetchRequests] = useAsync(
    useCallback(
      () =>
        retryWithRelogin(appContext, rootClusterUri, async () => {
          const {
            response: { requests },
          } = await tshd.getAccessRequests({
            clusterUri: rootClusterUri,
          });
          return requests;
        }),
      [tshd, rootClusterUri, appContext]
    )
  );

  return (
    <AccessRequestsContext.Provider
      value={{
        canUse,
        fetchRequestsAttempt,
        fetchRequests,
        assumed,
      }}
    >
      {children}
    </AccessRequestsContext.Provider>
  );
};

export const useAccessRequestsContext = () => {
  const context = useContext(AccessRequestsContext);

  if (!context) {
    throw new Error(
      'useAccessRequestsContext must be used within an AccessRequestsContext'
    );
  }

  return context;
};
