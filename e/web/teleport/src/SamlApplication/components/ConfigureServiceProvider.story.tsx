import { useState } from 'react';

import { Attempt } from 'shared/hooks/useAsync';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  emptySamlAppProps,
  MockSamlApplicationContextProvider,
  mockSamlMeta,
} from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { ContextProvider } from 'teleport/index';
import {
  SamlIdpServiceProvider,
  SamlServiceProviderPreset,
} from 'teleport/services/samlidp/types';

import {
  AddMetadataGeneric,
  ConfigureServiceProvider,
} from './ConfigureServiceProvider';

export default {
  title: 'TeleportE/SamlApplication/components/ConfigureServiceProvider',
  decorators: [
    Story => {
      const ctx = createTeleportContextE();
      return (
        <ContextProvider ctx={ctx}>
          <Story />
        </ContextProvider>
      );
    },
  ],
};

export const Default = () => {
  const [upsertRequest, setUpsertRequest] = useState(emptyUpsertRequest);
  return (
    <MockSamlApplicationContextProvider
      samlProviderProps={{
        ...emptySamlAppProps,
        upsertRequest,
        setUpsertRequest,
      }}
    >
      <ConfigureServiceProvider
        header="samlAppHeader"
        subtitle="samlAppSubtitle"
        agentMeta={mockSamlMeta}
        updateAgentMeta={() => null}
        prevStep={() => null}
        nextStep={() => null}
        SpMetadataConfigComponent={AddMetadataGeneric}
        preset={SamlServiceProviderPreset.Unspecified}
        isUpdateFlow={false}
      />
    </MockSamlApplicationContextProvider>
  );
};

export const Processing = () => {
  const upsertAttempt: Attempt<SamlIdpServiceProvider> = {
    status: 'processing',
    data: null,
    statusText: '',
  };
  return (
    <MockSamlApplicationContextProvider
      samlProviderProps={{
        ...emptySamlAppProps,
        upsertAttempt,
      }}
    >
      <ConfigureServiceProvider
        header="samlAppHeader"
        subtitle="samlAppSubtitle"
        agentMeta={mockSamlMeta}
        updateAgentMeta={() => null}
        prevStep={() => null}
        nextStep={() => null}
        SpMetadataConfigComponent={AddMetadataGeneric}
        preset={SamlServiceProviderPreset.Unspecified}
        isUpdateFlow={false}
      />
    </MockSamlApplicationContextProvider>
  );
};

export const Failed = () => {
  const upsertAttempt: Attempt<SamlIdpServiceProvider> = {
    status: 'error',
    error: new Error('failed to create service provider'),
    data: null,
    statusText: 'Failed to create service provider',
  };
  return (
    <MockSamlApplicationContextProvider
      samlProviderProps={{ ...emptySamlAppProps, upsertAttempt }}
    >
      <ConfigureServiceProvider
        header="samlAppHeader"
        subtitle="samlAppSubtitle"
        agentMeta={mockSamlMeta}
        updateAgentMeta={() => null}
        prevStep={() => null}
        nextStep={() => null}
        SpMetadataConfigComponent={AddMetadataGeneric}
        preset={SamlServiceProviderPreset.Unspecified}
        isUpdateFlow={false}
      />
    </MockSamlApplicationContextProvider>
  );
};
