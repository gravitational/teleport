import React, { useState } from 'react';

import { ButtonSecondary } from 'design/Button';

import Validation from 'shared/components/Validation';

import { AddMetadataGeneric } from './ConfigureServiceProvider';

export default {
  title: 'TeleportE/Discover/SAML Application/shared/AddMetadataGeneric',
};

export const Default = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: 'test',
    attributeMapping: [{ name: '', nameFormat: 'unspecified', value: '' }],
  });
  return (
    <Validation>
      <AddMetadataGeneric
        attempt={{ status: 'processing' }}
        spConfig={spConfig}
        setSPConfig={setSPConfig}
      />
    </Validation>
  );
};

export const FieldValidation = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: '',
    attributeMapping: [{ name: '', nameFormat: 'unspecified', value: '' }],
  });
  return (
    <Validation>
      {({ validator }) => (
        <>
          <AddMetadataGeneric
            attempt={{
              status: 'failed',
              statusText: 'application already exists',
            }}
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
