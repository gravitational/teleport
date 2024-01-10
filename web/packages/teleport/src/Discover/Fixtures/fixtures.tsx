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

import React, { PropsWithChildren } from 'react';
import { MemoryRouter } from 'react-router';

import { ResourceSpec } from 'teleport/Discover/SelectResource';
import { ContextProvider } from 'teleport';
import {
  DiscoverProvider,
  DiscoverContextState,
  AgentMeta,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ResourceKind } from 'teleport/Discover/Shared';
import cfg from 'teleport/config';
import { Acl, AuthType } from 'teleport/services/user';

export const TeleportProvider: React.FC<
  PropsWithChildren<{
    agentMeta: AgentMeta;
    resourceSpec?: ResourceSpec;
    interval?: number;
    customAcl?: Acl;
    authType?: AuthType;
    resourceKind: ResourceKind;
  }>
> = props => {
  const ctx = createTeleportContext({ customAcl: props.customAcl });
  if (props.authType) {
    ctx.storeUser.state.authType = props.authType;
  }
  const discoverCtx = defaultDiscoverContext({
    agentMeta: props.agentMeta,
    resourceSpec: props.resourceSpec,
  });

  return (
    <MemoryRouter initialEntries={[{ pathname: cfg.routes.discover }]}>
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>
            <PingTeleportProvider
              interval={props.interval || 100000}
              resourceKind={props.resourceKind}
            >
              {props.children}
            </PingTeleportProvider>
          </DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

export function defaultDiscoverContext({
  agentMeta,
  resourceSpec,
}: {
  agentMeta?: AgentMeta;
  resourceSpec?: ResourceSpec;
}): DiscoverContextState {
  return {
    agentMeta: agentMeta
      ? agentMeta
      : { resourceName: '', agentMatcherLabels: [] },
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: resourceSpec ? resourceSpec : defaultResourceSpec(null),
  };
}

export function defaultResourceSpec(kind: ResourceKind): ResourceSpec {
  return {
    name: '',
    kind,
    icon: null,
    keywords: '',
    event: null,
  };
}
