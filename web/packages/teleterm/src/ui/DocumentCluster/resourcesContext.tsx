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

import React, {
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
  useRef,
} from 'react';

export interface ResourcesContext {
  /**
   * requestResourcesRefresh makes all DocumentCluster instances within the workspace refresh the
   * resource list with current filters.
   *
   * Its main purpose is to refresh the resource list in existing DocumentCluster tabs after a
   * Connect My Computer node is set up.
   */
  requestResourcesRefresh: () => void;
  /**
   * onResourcesRefreshRequest registers a listener that will be called any time a refresh is
   * requested. Typically called from useEffect, for this purpose it returns a cleanup function.
   */
  onResourcesRefreshRequest: (listener: () => void) => { cleanup: () => void };
}

const ResourcesContext = createContext<ResourcesContext>(null);

export const ResourcesContextProvider: FC<PropsWithChildren> = props => {
  const emitterRef = useRef<EventEmitter>();
  if (!emitterRef.current) {
    emitterRef.current = new EventEmitter();
  }

  // This function could be expanded to emit a cluster URI so that a request refresh for a root
  // cluster doesn't trigger refreshes of leaf DocumentCluster instances and vice versa.
  // However, the implementation should be good enough for now since it's used only in Connect My
  // Computer setup anyway.
  const requestResourcesRefresh = useCallback(
    () => emitterRef.current.emit('refresh'),
    []
  );

  const onResourcesRefreshRequest = useCallback(listener => {
    emitterRef.current.addListener('refresh', listener);

    return {
      cleanup: () => {
        emitterRef.current.removeListener('refresh', listener);
      },
    };
  }, []);

  return (
    <ResourcesContext.Provider
      value={{ requestResourcesRefresh, onResourcesRefreshRequest }}
      children={props.children}
    />
  );
};

export const useResourcesContext = () => {
  const context = useContext(ResourcesContext);

  if (!context) {
    throw new Error(
      'useResourcesContext must be used within a ResourcesContextProvider'
    );
  }

  return context;
};
