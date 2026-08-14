import { useState } from 'react';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  emptySamlAppProps,
  MockSamlApplicationContextProvider,
  resourceSpecSamlGeneric,
} from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { microsoftEntraIdPresetSpec } from 'e-teleport/services/idp/types';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';

import { AddEntraSpToTeleport as AddEntraSpToTeleportComponent } from './AddEntraSpToTeleport';

export default {
  title: 'TeleportE/SamlApplication/MicrosoftEntraId',
};

export const AddEntraSpToTeleport = () => {
  const ctx = createTeleportContextE();
  const req = emptyUpsertRequest;
  const msEntraPreset = microsoftEntraIdPresetSpec(
    'e14205e2-0342-4d9b-9a00-60f7bc19648b'
  );
  req.attributeMapping = msEntraPreset.attribute_mapping;
  req.entityID = msEntraPreset.entity_id;
  req.acsURL = msEntraPreset.acs_url;
  const [upsertRequest, setUpsertRequest] = useState(req);
  return (
    <RequiredDiscoverProviders
      agentMeta={{}}
      resourceSpec={resourceSpecSamlGeneric}
      teleportCtx={ctx}
    >
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          ...emptySamlAppProps,
          upsertRequest,
          setUpsertRequest,
          guidedToggle: true,
          guidedConfig: {
            samlMicrosoftEntraId: {
              tenantId: '',
            },
          },
        }}
      >
        <AddEntraSpToTeleportComponent />
      </MockSamlApplicationContextProvider>
    </RequiredDiscoverProviders>
  );
};
