/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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

    // TODO(lisa): validate thumbprint with the back.
    // This is a nice to have, so not a blocker.
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
              text: `https://${props.clusterPublicUri}`,
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
