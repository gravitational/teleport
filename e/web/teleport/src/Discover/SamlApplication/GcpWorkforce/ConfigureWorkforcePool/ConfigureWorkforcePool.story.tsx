import { useState } from 'react';

import {
  idpMetadata,
  MockSamlApplicationContextProvider,
} from 'e-teleport/SamlApplication/fixtures';
import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';

import {
  ConfigurePool,
  ConfigurePoolProps,
  defaultSamlMetaForGcpWorkforce,
} from './ConfigureWorkforcePool';

export default {
  title: 'TeleportE/Discover/SAML Application/GCP Workforce',
};

export const ConfigureWorkforcePool = () => {
  const [upsertRequest, setUpsertRequest] = useState(emptyUpsertRequest);
  const [guidedToggle, setGuidedToggle] = useState(null);
  const [guidedConfig, setGuidedConfig] = useState(
    defaultSamlMetaForGcpWorkforce
  );

  const fetchMetadataValuesAttempt = {
    status: 'success',
    data: idpMetadata,
    statusText: '',
  };

  return (
    <MockSamlApplicationContextProvider
      samlProviderProps={{
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
  );
};

const props: ConfigurePoolProps = {
  nextStep: () => null,
  prevStep: () => null,
};
