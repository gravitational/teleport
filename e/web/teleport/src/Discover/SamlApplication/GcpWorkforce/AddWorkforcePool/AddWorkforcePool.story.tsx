import { useState } from 'react';
import { MemoryRouter } from 'react-router';
import cfg from 'teleport/config';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';
import { ContextProvider } from 'teleport';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { MockSamlApplicationContextProvider } from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';

import { defaultSamlMetaForGcpWorkforce } from '../ConfigureWorkforcePool/ConfigureWorkforcePool';

import { Container as AddWorkforcePoolToTeleport } from './AddWorkforcePool';

import type { SamlGcpWorkforce } from 'teleport/services/samlidp/types';

export default {
  title: 'TeleportE/Discover/SAML Application/GCP Workforce',
};

export const AddWorkforcePool = () => {
  const [upsertRequest, setUpsertRequest] = useState(emptyUpsertRequest);
  return (
    <DiscoverContextProvider>
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          upsertRequest,
          setUpsertRequest,
        }}
      >
        <AddWorkforcePoolToTeleport />
      </MockSamlApplicationContextProvider>
    </DiscoverContextProvider>
  );
};

export const AddWorkforcePoolDisabledOnGuided = () => {
  const [upsertRequest, setUpsertRequest] = useState(emptyUpsertRequest);
  return (
    <DiscoverContextProvider>
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          upsertRequest,
          setUpsertRequest,
          guidedToggle: true,
          guidedConfig: defaultSamlMetaForGcpWorkforce,
        }}
      >
        <AddWorkforcePoolToTeleport />
      </MockSamlApplicationContextProvider>
    </DiscoverContextProvider>
  );
};

const DiscoverContextProvider = props => {
  const ctx = createTeleportContextE({ customAcl: props.customAcl });
  const discoverCtx: DiscoverContextState = {
    ...props,
    currentStep: 0,
    onSelectResource: () => null,
    resourceSpec: {
      samlMeta: { preset: SamlServiceProviderPreset.GcpWorkforce },
    },
    agentMeta: {
      orgId: '123456',
      poolName: 'test-pool-name',
      poolProviderName: 'test-provider-name',
    } as SamlGcpWorkforce,
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
