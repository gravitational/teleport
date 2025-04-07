import { useState } from 'react';

import { ButtonSecondary } from 'design/Button';
import Validation from 'shared/components/Validation';

import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

import { AddMetadataGeneric } from './ConfigureServiceProvider';

export default {
  title: 'TeleportE/SamlApplication/components/AddMetadataGeneric',
};

export const Default = () => {
  const [spConfig, setSPConfig] =
    useState<CreateSamlIdpServiceProviderRequest>(emptyUpsertRequest);
  return (
    <Validation>
      <AddMetadataGeneric
        {...props}
        spConfig={spConfig}
        setSPConfig={setSPConfig}
      />
    </Validation>
  );
};

export const FieldValidation = () => {
  const [spConfig, setSPConfig] =
    useState<CreateSamlIdpServiceProviderRequest>(emptyUpsertRequest);
  return (
    <Validation>
      {({ validator }) => (
        <>
          <AddMetadataGeneric
            {...props}
            spConfig={spConfig}
            setSPConfig={setSPConfig}
          />
          <ButtonSecondary
            mt={6}
            onClick={() => {
              if (!validator.validate()) {
                return;
              }
            }}
          >
            Test Validation
          </ButtonSecondary>
        </>
      )}
    </Validation>
  );
};

export const DisabledOnUpdate = () => {
  return (
    <Validation>
      <AddMetadataGeneric {...props} isProcessing={true} />
    </Validation>
  );
};

const props = {
  spConfig: emptyUpsertRequest,
  setSPConfig: () => null,
  isProcessing: false,
  isGuided: false,
  isUpdateFlow: false,
  setLabelsValidator: () => null,
  preset: SamlServiceProviderPreset.Unspecified,
};
