import React, { useState } from 'react';
import { useRule } from 'shared/components/Validation';
import TextEditor from 'shared/components/TextEditor';
import { ButtonTextWithAddIcon } from 'shared/components/ButtonTextWithAddIcon';
import { LabelInput, Flex } from 'design';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

export function AddEntityDescriptor({
  spConfig,
  setSPConfig,
}: {
  spConfig: CreateSamlIdpServiceProviderRequest;
  setSPConfig: (SPConfig) => void;
}) {
  const [showEntityDescriptor, setShowEntityDescriptor] = useState(false);
  return (
    <>
      <ButtonTextWithAddIcon
        label="Add Entity Descriptor (optional)"
        onClick={() => setShowEntityDescriptor(!showEntityDescriptor)}
        disabled={false}
      />
      {showEntityDescriptor && (
        <EntityDescriptorEditor spConfig={spConfig} setSPConfig={setSPConfig} />
      )}
    </>
  );
}

function EntityDescriptorEditor({
  spConfig,
  setSPConfig,
}: {
  spConfig: CreateSamlIdpServiceProviderRequest;
  setSPConfig: (SPConfig) => void;
}) {
  const { valid, message } = useRule(
    validateEntityDescriptor(spConfig.entityDescriptor)
  );
  const hasError = !valid;
  const labelText = hasError
    ? message
    : `Paste Service Provider entity descriptor's XML content. Please refer to your Service Provider's
    documentation for instructions on how to obtain the entity descriptor.`;

  return (
    <LabelInput mt={3} hasError={hasError}>
      {labelText}
      <Flex
        height="320px"
        mt={2}
        borderRadius={2}
        border={hasError ? '2px solid' : '1px solid'}
        borderColor={hasError ? 'error.main' : 'levels.sunken'}
      >
        <TextEditor
          readOnly={false}
          bg="levels.deep"
          data={[{ content: spConfig.entityDescriptor }]}
          onChange={v => setSPConfig({ ...spConfig, entityDescriptor: v })}
        />
      </Flex>
    </LabelInput>
  );
}

const validateEntityDescriptor = (value: string) => () => {
  // Basic validation of the XML to make sure the entity descriptor starts with < and ends with >
  if (value?.length > 0) {
    if (!value.startsWith('<') || !value.endsWith('>')) {
      return {
        valid: false,
        message: 'Entity descriptor XML must start with < and end with >',
      };
    }
  }
  return { valid: true };
};
