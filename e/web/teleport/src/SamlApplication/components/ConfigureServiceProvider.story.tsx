import { useState } from 'react';

import {
  MockSamlApplicationContextProvider,
  mockSamlMeta,
} from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import {
  AddMetadataGeneric,
  ConfigureServiceProvider,
} from './ConfigureServiceProvider';

export default {
  title: 'TeleportE/SamlApplication/components/ConfigureServiceProvider',
};

export const Default = () => {
  const [upsertRequest, setUpsertRequest] = useState(emptyUpsertRequest);
  return (
    <MockSamlApplicationContextProvider
      samlProviderProps={{ upsertRequest, setUpsertRequest }}
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
  const upsertAttempt = {
    status: 'processing',
    data: null,
    statusText: '',
  };
  return (
    <MockSamlApplicationContextProvider samlProviderProps={{ upsertAttempt }}>
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
  const upsertAttempt = {
    status: 'error',
    data: null,
    statusText: 'Failed to create service provider',
  };
  return (
    <MockSamlApplicationContextProvider samlProviderProps={{ upsertAttempt }}>
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
