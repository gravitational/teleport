import { useState } from 'react';

import { Attempt } from 'shared/hooks/useAsync';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import {
  emptySamlAppProps,
  idpMetadata,
  MockSamlApplicationContextProvider,
} from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { SamlIdpMetadataResponse } from 'e-teleport/services/idp';
import { ContextProvider } from 'teleport/index';

import {
  ConfigurePool,
  ConfigurePoolProps,
  defaultSamlMetaForGcpWorkforce,
} from './ConfigureWorkforcePool';

export default {
  title: 'TeleportE/Discover/SAML Application/GCP Workforce',
};

const ctx = createTeleportContextE();

export const ConfigureWorkforcePool = () => {
  const [upsertRequest, setUpsertRequest] = useState(emptyUpsertRequest);
  const [guidedToggle, setGuidedToggle] = useState(null);
  const [guidedConfig, setGuidedConfig] = useState(
    defaultSamlMetaForGcpWorkforce
  );

  const fetchMetadataValuesAttempt: Attempt<SamlIdpMetadataResponse> = {
    status: 'success',
    data: idpMetadata,
    statusText: '',
  };

  return (
    <ContextProvider ctx={ctx}>
      <MockSamlApplicationContextProvider
        samlProviderProps={{
          ...emptySamlAppProps,
          fetchMetadataValuesAttempt,
          upsertRequest,
          setUpsertRequest,
          guidedToggle,
          setGuidedToggle,
          guidedConfig,
          setGuidedConfig,
        }}
      >
        <ConfigurePool {...props} />
      </MockSamlApplicationContextProvider>
    </ContextProvider>
  );
};

const props: ConfigurePoolProps = {
  nextStep: () => null,
  prevStep: () => null,
};
