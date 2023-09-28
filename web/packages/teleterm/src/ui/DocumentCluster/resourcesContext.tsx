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

import { EventEmitter } from 'events';

import React, {
  createContext,
  FC,
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

export const ResourcesContextProvider: FC = props => {
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
