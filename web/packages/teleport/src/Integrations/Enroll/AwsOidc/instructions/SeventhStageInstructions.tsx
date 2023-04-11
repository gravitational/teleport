import React, { useCallback, useState } from 'react';

import Text from 'design/Text';
import Box from 'design/Box';

import { ButtonPrimary } from 'design';

import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';

import { requiredField } from 'shared/components/Validation/rules';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

export function SeventhStageInstructions(props: CommonInstructionsProps) {
  const [roleARN, setRoleARN] = useState('');
  const [name, setName] = useState('');

  const handleSubmit = useCallback(() => {
    // TODO: create the integration
  }, [roleARN]);

  return (
    <InstructionsContainer>
      <Text>From the list of roles, select the role you just created</Text>

      <Text mt={5}>Copy the role ARN and paste it below</Text>

      <Validation>
        {({ validator }) => (
          <>
            <Box mt={5}>
              <FieldInput
                onChange={e => setRoleARN(e.target.value)}
                value={roleARN}
                placeholder="Role ARN"
                rule={requiredField('Role ARN is required')}
              />
            </Box>
            <Text mt={5}>Give this AWS integration a name</Text>
            <Box mt={5}>
              <FieldInput
                onChange={e => setName(e.target.value)}
                value={name}
                placeholder="Integration name"
                rule={requiredField('Name is required')}
              />
            </Box>
            <Box mt={5}>
              <ButtonPrimary onClick={() => handleSubmit()}>Next</ButtonPrimary>
            </Box>
          </>
        )}
      </Validation>
    </InstructionsContainer>
  );
}
