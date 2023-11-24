/**
 * Copyright 2022 Gravitational, Inc.
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

import React from 'react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { nodes } from 'teleport/Nodes/fixtures';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  DiscoverProvider,
  DiscoverContextState,
} from 'teleport/Discover/useDiscover';

import { TestConnection } from './TestConnection';

export default {
  title: 'Teleport/Discover/Shared/ConnectionDiagnostic/Server',
};

const node = { ...nodes[0] };
node.sshLogins = [
  ...node.sshLogins,
  'george_washington_really_long_name_testing',
];
const agentStepProps = {
  prevStep: () => {},
  nextStep: () => {},
  agentMeta: { resourceName: node.hostname, node, agentMatcherLabels: [] },
};

export const Init = () => (
  <Provider>
    <TestConnection {...agentStepProps} />
  </Provider>
);

const Provider = ({ children }) => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    ...agentStepProps,
    currentStep: 0,
    onSelectResource: () => null,
    resourceSpec: undefined,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'server' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>{children}</DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
