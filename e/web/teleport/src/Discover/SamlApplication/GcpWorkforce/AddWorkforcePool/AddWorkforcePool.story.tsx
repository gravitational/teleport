import { PropsWithChildren, useState } from 'react';

import {
  emptySamlAppProps,
  MockSamlApplicationContextProvider,
  resourceSpecSamlGeneric,
} from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { gcpWorkforcePresetSpec } from 'e-teleport/services/idp/types';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';

import { defaultSamlMetaForGcpWorkforce } from '../ConfigureWorkforcePool/ConfigureWorkforcePool';
import { Container as AddWorkforcePoolToTeleport } from './AddWorkforcePool';

export default {
  title: 'TeleportE/Discover/SAML Application/GCP Workforce',
};

export const AddWorkforcePool = () => {
  const [upsertRequest, setUpsertRequest] = useState(emptyUpsertRequest);
  return (
    <DiscoverContextProvider>
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          ...emptySamlAppProps,
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
  const gcpPreset = emptyUpsertRequest;
  gcpPreset.attributeMapping = gcpWorkforcePresetSpec().attribute_mapping;
  const [upsertRequest, setUpsertRequest] = useState(gcpPreset);
  return (
    <DiscoverContextProvider>
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          ...emptySamlAppProps,
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

const DiscoverContextProvider: React.FC<PropsWithChildren> = props => {
  return (
    <RequiredDiscoverProviders
      agentMeta={{
        samlGcpWorkforce: {
          orgId: '123456',
          poolName: 'test-pool-name',
          poolProviderName: 'test-provider-name',
        },
      }}
      resourceSpec={resourceSpecSamlGeneric}
    >
      {props.children}
    </RequiredDiscoverProviders>
  );
};
