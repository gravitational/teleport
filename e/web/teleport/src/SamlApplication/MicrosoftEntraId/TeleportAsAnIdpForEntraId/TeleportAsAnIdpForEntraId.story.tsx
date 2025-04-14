import { useState } from 'react';

import { Attempt } from 'shared/hooks/useAsync';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  emptySamlAppProps,
  idpMetadata,
  MockSamlApplicationContextProvider,
  resourceSpecSamlGeneric,
} from 'e-teleport/SamlApplication/fixtures';
import { SamlIdpMetadataResponse } from 'e-teleport/services/idp';
import { RequiredDiscoverProviders } from 'teleport/Discover/Fixtures/fixtures';

import { TeleportAsAnIdpForEntraId as TeleportAsAnIdpForEntraIdComponent } from './TeleportAsAnIdpForEntraId';

export default {
  title: 'TeleportE/SamlApplication/MicrosoftEntraId',
};

export const TeleportAsAnIdpForEntraId = () => {
  const ctx = createTeleportContextE();
  ctx.idpService.getIdPMetadataValues = () => Promise.resolve(idpMetadata);
  ctx.idpService.getMetadataXml = () => Promise.resolve('<xml>test<xml>');
  const fetchMetadataValuesAttempt: Attempt<SamlIdpMetadataResponse> = {
    status: 'success',
    data: idpMetadata,
    statusText: '',
  };
  const [guidedConfig, setGuidedConfig] = useState({
    samlMicrosoftEntraId: {
      tenantId: '',
    },
  });
  return (
    <RequiredDiscoverProviders
      agentMeta={{}}
      resourceSpec={resourceSpecSamlGeneric}
      teleportCtx={ctx}
    >
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          ...emptySamlAppProps,
          fetchMetadataValuesAttempt,
          guidedConfig,
          setGuidedConfig,
          guidedToggle: true,
        }}
      >
        <TeleportAsAnIdpForEntraIdComponent />
      </MockSamlApplicationContextProvider>
    </RequiredDiscoverProviders>
  );
};
