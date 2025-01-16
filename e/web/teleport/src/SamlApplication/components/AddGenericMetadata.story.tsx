import { ButtonSecondary } from 'design/Button';
import Validation from 'shared/components/Validation';

import { emptyUpsertRequest } from 'e-teleport/SamlApplication/hooks/useSamlApplication';

import { AddMetadataGeneric } from './ConfigureServiceProvider';

export default {
  title: 'TeleportE/SamlApplication/components/AddMetadataGeneric',
};

export const Default = () => {
  return (
    <Validation>
      <AddMetadataGeneric disableInputs={false} {...props} />
    </Validation>
  );
};

export const FieldValidation = () => {
  return (
    <Validation>
      {({ validator }) => (
        <>
          <AddMetadataGeneric disableInputs={false} {...props} />
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

export const Disabled = () => {
  return (
    <Validation>
      <AddMetadataGeneric disableInputs={true} {...props} />
    </Validation>
  );
};

const props = {
  spConfig: emptyUpsertRequest,
  setSPConfig: () => null,
  isUpdateFlow: false,
  setLabelsValidator: () => null,
};
