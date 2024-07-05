import React, { useState } from 'react';
import { MemoryRouter } from 'react-router';

import cfg from 'teleport/config';

import {
  DiscoverProvider,
  DiscoverContextState,
} from 'teleport/Discover/useDiscover';

import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { ContextProvider } from 'teleport';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { ConfigurePool, ConfigurePoolProps } from './ConfigureWorkforcePool';

import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';

import type { SamlGcpWorkforce } from 'teleport/services/samlidp/types';

export default {
  title: 'TeleportE/Discover/SAML Application/GCP Workforce',
};

export const ConfigureWorkforcePool = () => {
  const [agentMeta, setAgentMeta] = useState<SamlGcpWorkforce>({
    isAutoConfig: true,
    orgId: '',
    poolName: '',
    poolProviderName: '',
  });

  return (
    <Provider>
      <ConfigurePool
        agentMeta={agentMeta}
        {...props}
        updateAgentMeta={setAgentMeta}
      />
    </Provider>
  );
};

const Provider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });
  const discoverCtx: DiscoverContextState = {
    ...props,
    currentStep: 0,
    onSelectResource: () => null,
    resourceSpec: undefined,
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

const props: ConfigurePoolProps = {
  nextStep: () => null,
  prevStep: () => null,
  fetchMetadata: () => Promise.resolve({} as SAMLIdPMetadataResponse),
  updateAgentMeta: SamlGcpWorkforce => SamlGcpWorkforce,
  resourceSpec: {
    samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
  } as ResourceSpec,
};
