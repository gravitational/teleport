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

import { EventEmitter } from 'events';

import {
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
  useRef,
} from 'react';

import { RootClusterUri } from 'teleterm/ui/uri';

export interface ResourcesContext {
  /**
   * requestResourcesRefresh makes all DocumentCluster instances within the workspace
   * (specified by `rootClusterUri`) refresh the resource list with current filters.
   *
   * Its purpose is to refresh the resource list in existing DocumentCluster tabs after a
   * Connect My Computer node is set up, or after assuming/dropping an access request.
   */
  requestResourcesRefresh: (rootClusterUri: RootClusterUri) => void;
  /**
   * onResourcesRefreshRequest registers a listener that will be called any time a refresh is
   * requested for a particular rootClusterUri. Typically called from useEffect, for this purpose it
   * returns a cleanup function.
   */
  onResourcesRefreshRequest: (
    rootClusterUri: RootClusterUri,
    listener: () => void
  ) => {
    cleanup: () => void;
  };
}

const ResourcesContext = createContext<ResourcesContext>(null);

export const ResourcesContextProvider: FC<PropsWithChildren> = props => {
  const emitterRef = useRef<EventEmitter>();
  if (!emitterRef.current) {
    emitterRef.current = new EventEmitter();
  }

  const requestResourcesRefresh = useCallback(
    (rootClusterUri: RootClusterUri) =>
      emitterRef.current.emit('refresh', rootClusterUri),
    []
  );

  const onResourcesRefreshRequest = useCallback(
    (
      targetRootClusterUri: RootClusterUri,
      listenerWithoutRootClusterUri: () => void
    ) => {
      const listener = (rootClusterUri: RootClusterUri) => {
        if (rootClusterUri === targetRootClusterUri) {
          listenerWithoutRootClusterUri();
        }
      };
      emitterRef.current.addListener('refresh', listener);

      return {
        cleanup: () => {
          emitterRef.current.removeListener('refresh', listener);
        },
      };
    },
    []
  );

  return (
    <ResourcesContext.Provider
      value={{ requestResourcesRefresh, onResourcesRefreshRequest }}
      children={props.children}
    />
  );
};

export const useResourcesContext = (rootClusterUri: RootClusterUri) => {
  const context = useContext(ResourcesContext);

  if (!context) {
    throw new Error(
      'useResourcesContext must be used within a ResourcesContextProvider'
    );
  }

  const {
    requestResourcesRefresh: requestResourcesRefreshContext,
    onResourcesRefreshRequest: onResourcesRefreshRequestContext,
  } = context;

  return {
    requestResourcesRefresh: useCallback(
      () => requestResourcesRefreshContext(rootClusterUri),
      [requestResourcesRefreshContext, rootClusterUri]
    ),
    onResourcesRefreshRequest: useCallback(
      (listener: () => void) =>
        onResourcesRefreshRequestContext(rootClusterUri, listener),
      [onResourcesRefreshRequestContext, rootClusterUri]
    ),
  };
};
