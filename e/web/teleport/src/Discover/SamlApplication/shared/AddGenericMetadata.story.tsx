import React, { useState } from 'react';

import { ButtonSecondary } from 'design/Button';

import Validation from 'shared/components/Validation';

import { ResourceKind } from 'teleport/Discover/Shared';

import { AddMetadataGeneric } from './ConfigureServiceProvider';

import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';

export default {
  title: 'TeleportE/Discover/SAML Application/shared/AddMetadataGeneric',
};

export const Default = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: 'test',
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  });
  return (
    <Validation>
      <AddMetadataGeneric
        attempt={{ status: 'processing' }}
        spConfig={spConfig}
        setSPConfig={setSPConfig}
        resourceSpec={{ kind: ResourceKind.SamlApplication } as ResourceSpec}
        isUpdateFlow={false}
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
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
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
            resourceSpec={
              { kind: ResourceKind.SamlApplication } as ResourceSpec
            }
            isUpdateFlow={false}
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
