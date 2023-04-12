import React, { useState } from 'react';

import Text from 'design/Text';
import Box from 'design/Box';

import { ButtonPrimary } from 'design';

import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';

import { requiredField } from 'shared/components/Validation/rules';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

export function SecondStageInstructions(props: CommonInstructionsProps) {
  const [thumbprint, setThumbprint] = useState('');

  function handleSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    // TODO(lisa): validate thumbprint
    props.onNext();
  }

  return (
    <InstructionsContainer>
      <Text>
        Now select <strong>OpenID Connect</strong>
      </Text>

      <Text mt={5}>
        Copy the following into <strong>Provider URL</strong>
      </Text>

      <Box mt={5}>
        <TextSelectCopyMulti
          bash={false}
          lines={[
            {
              text: 'https://teleport.lol', // TODO: replace all instances of this URL with the actual hostname:port
            },
          ]}
        />
      </Box>

      <Text mt={5}>
        Copy the following into <strong>Audience</strong>
      </Text>

      <Box mt={5}>
        <TextSelectCopyMulti
          bash={false}
          lines={[
            {
              text: 'discover.teleport',
            },
          ]}
        />
      </Box>

      <Text mt={5}>
        Then, click <strong>Get thumbprint</strong>
      </Text>

      <Text mt={5}>Paste the thumbprint below for verification</Text>

      <Validation>
        {({ validator }) => (
          <>
            <Box mt={2}>
              <FieldInput
                label="thumbprint"
                onChange={e => setThumbprint(e.target.value)}
                value={thumbprint}
                placeholder="Paste the thumbprint here"
                rule={requiredField('Thumbprint is required')}
              />
            </Box>
            <Box mt={5}>
              <ButtonPrimary onClick={() => handleSubmit(validator)}>
                Next
              </ButtonPrimary>
            </Box>
          </>
        )}
      </Validation>
    </InstructionsContainer>
  );
}
