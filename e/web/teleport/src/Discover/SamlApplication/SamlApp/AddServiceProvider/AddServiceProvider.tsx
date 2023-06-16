import React, { useState, useEffect } from 'react';
import { Box, LabelInput, Flex } from 'design';
import { Danger } from 'design/Alert';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import FieldInput from 'shared/components/FieldInput';
import Validation, { useRule, Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import TextEditor from 'shared/components/TextEditor';

import { AgentMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  HeaderSubtitle,
  Header,
  ActionButtons,
} from 'teleport/Discover/Shared';

import useTeleportE from 'e-teleport/useTeleportE';

export function Container() {
  const { idpService } = useTeleportE();
  const { attempt, run } = useAttempt('');
  const { prevStep, nextStep, updateAgentMeta, agentMeta } = useDiscover();

  function onSubmit(
    validator: Validator,
    name: string,
    entityDescriptor: string
  ) {
    if (!validator.validate()) {
      return;
    }
    run(() =>
      idpService.createSamlIdpServiceProvider({ name, entityDescriptor })
    );
  }

  return (
    <AddServiceProvider
      attempt={attempt}
      onSubmit={onSubmit}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
    />
  );
}

export function AddServiceProvider({
  attempt,
  onSubmit,
  agentMeta,
  updateAgentMeta,
  nextStep,
  prevStep,
}: Props) {
  const [name, setName] = useState('');
  const [entityDescriptor, setEntityDescriptor] = useState('');

  useEffect(() => {
    if (attempt.status === 'success') {
      updateAgentMeta({ ...agentMeta, resourceName: name });
      nextStep();
    }
  }, [attempt, nextStep]);

  return (
    <>
      <Header>Add Your Service Provider To Teleport</Header>
      <HeaderSubtitle>
        Enter a name for the SAML integration and paste your service provider's
        entity descriptor's XML content. Please refer to your service provider's
        documentation for instructions on how to obtain the entity descriptor.
      </HeaderSubtitle>
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      <Box maxWidth="800px">
        <Validation>
          {({ validator }) => (
            <>
              <FieldInput
                mb={3}
                rule={requiredField('Name is required')}
                label="Name"
                autoFocus
                value={name}
                placeholder="app_saml"
                width="240px"
                mr="3"
                onChange={e => setName(e.target.value)}
                disabled={attempt.status === 'processing'}
              />
              <EntityDescriptorInput
                entityDescriptor={entityDescriptor}
                setEntityDescriptor={setEntityDescriptor}
              />
              <ActionButtons
                onProceed={() => onSubmit(validator, name, entityDescriptor)}
                disableProceed={attempt.status === 'processing'}
                onPrev={prevStep}
                lastStep
              />
            </>
          )}
        </Validation>
      </Box>
    </>
  );
}

export function EntityDescriptorInput({
  entityDescriptor,
  setEntityDescriptor,
}: {
  entityDescriptor: string;
  setEntityDescriptor: (string) => void;
}) {
  const { valid, message } = useRule(
    validateEntityDescriptor(entityDescriptor)
  );
  const hasError = !valid;
  const labelText = hasError ? message : 'Entity Descriptor';

  return (
    <>
      <LabelInput mt={3} hasError={hasError}>
        {labelText}
      </LabelInput>
      <Flex
        height="320px"
        mt={2}
        border={hasError ? '2px solid' : '1px solid'}
        borderColor={hasError ? 'error.dark' : 'levels.surface'}
      >
        <TextEditor
          readOnly={false}
          data={[{ content: entityDescriptor }]}
          onChange={setEntityDescriptor}
        />
      </Flex>
    </>
  );
}

const validateEntityDescriptor = (value: string) => () => {
  if (!value) {
    return {
      valid: false,
      message: 'Entity descriptor is required',
    };
  }
  // Basic validation of the XML to make sure the entity descriptor starts with < and ends with >
  if (!value.startsWith('<') || !value.endsWith('>')) {
    return {
      valid: false,
      message: 'Entity descriptor XML is invalid',
    };
  }
  return { valid: true };
};

export type Props = {
  attempt: Attempt;
  agentMeta: AgentMeta;
  updateAgentMeta: (meta: AgentMeta) => void;
  prevStep: () => void;
  nextStep: () => void;
  onSubmit: (
    validator: Validator,
    name: string,
    entityDescriptor: string
  ) => void;
};
