import React from 'react';
import { MemoryRouter } from 'react-router';

import cfg from 'teleport/config';

import {
  DiscoverProvider,
  DiscoverContextState,
  SamlGcpWorkforceMeta,
} from 'teleport/Discover/useDiscover';

import { SamlServiceProviderPreset } from 'teleport/Discover/SelectResource/types';

import { ContextProvider } from 'teleport';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { Container as AddWorkforcePoolToTeleport } from './AddWorkforcePool';

export default {
  title: 'TeleportE/Discover/SAML Application/GCP Workforce',
};

export const AddWorkforcePool = () => {
  return (
    <Provider>
      <AddWorkforcePoolToTeleport />
    </Provider>
  );
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });
  const discoverCtx: DiscoverContextState = {
    ...props,
    currentStep: 0,
    onSelectResource: () => null,
    resourceSpec: {
      samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
    },
    agentMeta: {
      isAutoConfig: true,
      orgId: '123456',
      poolName: 'test-pool-name',
      poolProviderName: 'test-provider-name',
    } as SamlGcpWorkforceMeta,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'app' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>
          {props.children}
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
