/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
    nextStep: jest.fn(),
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
