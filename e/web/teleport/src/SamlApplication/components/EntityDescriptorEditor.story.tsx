import { useState } from 'react';

import { ButtonSecondary } from 'design/Button';
import Validation, { Validator } from 'shared/components/Validation';

import {
  AddEntityDescriptor,
  EntityDescriptorEditor,
} from './EntityDescriptorEditor';

export default {
  title: 'TeleportE/SamlApplication/components/EntityDescriptor',
};

export const Default = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: '',
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  });

  return (
    <Validation>
      <AddEntityDescriptor spConfig={spConfig} setSPConfig={setSPConfig} />
    </Validation>
  );
};

export const Editor = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: '',
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  });

  return (
    <Validation>
      <EntityDescriptorEditor spConfig={spConfig} setSPConfig={setSPConfig} />
    </Validation>
  );
};

export const EditorValidation = () => {
  const [spConfig, setSPConfig] = useState({
    name: '',
    entityID: '',
    acsURL: '',
    entityDescriptor: 'test',
    attributeMapping: [{ name: '', name_format: 'unspecified', value: '' }],
  });

  const [validator] = useState(() => new Validator());
  validator.validate();

  return (
    <Validation>
      {({ validator }) => (
        <>
          <EntityDescriptorEditor
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
