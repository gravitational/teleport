/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, {
  useContext,
  FC,
  createContext,
  useState,
  useEffect,
} from 'react';

import { RootClusterUri } from 'teleterm/ui/uri';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import type { AgentProcessState } from 'teleterm/mainProcess/types';

export interface ConnectMyComputerContext {
  state: AgentProcessState;
}

const ConnectMyComputerContext = createContext<ConnectMyComputerContext>(null);

export const ConnectMyComputerContextProvider: FC<{
  rootClusterUri: RootClusterUri;
}> = props => {
  const { mainProcessClient } = useAppContext();
  const [agentState, setAgentState] = useState<AgentProcessState>(() => ({
    status: 'not-started',
  }));

  useEffect(() => {
    const { cleanup } = mainProcessClient.subscribeToAgentUpdate(
      (rootClusterUri, state) => {
        if (props.rootClusterUri === rootClusterUri) {
          setAgentState(state);
        }
      }
    );
    return cleanup;
  }, [mainProcessClient, props.rootClusterUri]);

  return (
    <ConnectMyComputerContext.Provider
      value={{ state: agentState }}
      children={props.children}
    />
  );
};

export const useConnectMyComputerContext = () => {
  const context = useContext(ConnectMyComputerContext);

  if (!context) {
    throw new Error(
      'ConnectMyComputerContext requires ConnectMyComputerContextProvider context.'
    );
  }

  return context;
};
