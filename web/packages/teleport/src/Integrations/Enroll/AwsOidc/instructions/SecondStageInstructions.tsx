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
import { ButtonPrimary, ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';

import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';
import { requiredField } from 'shared/components/Validation/rules';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { integrationService } from 'teleport/services/integrations';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps, PreviousStepProps } from './common';

export function SecondStageInstructions(
  props: CommonInstructionsProps & PreviousStepProps
) {
  const [thumbprint, setThumbprint] = useState(props.awsOidc.thumbprint);
  const { attempt, run } = useAttempt();

  function handleSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() =>
      integrationService.fetchThumbprint().then(fetchedThumbprint => {
        if (thumbprint === fetchedThumbprint) {
          props.onNext({ ...props.awsOidc, thumbprint });
          return;
        }

        // the wrapper `run` will catch this error and
        // set the attempt to failed.
        throw new Error(
          `the thumbprint provided is incorrect, make sure\
          you copied the correct thumbprint from the AWS page`
        );
      })
    );
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
                mb={1}
                autoFocus
                label="thumbprint"
                onChange={e => setThumbprint(e.target.value)}
                value={thumbprint}
                placeholder="Paste the thumbprint here"
                rule={requiredField('Thumbprint is required')}
                markAsError={attempt.status === 'failed'}
              />
            </Box>
            {attempt.status === 'failed' && (
              <Text color="error.main">
                <Icons.Warning mr={2} color="error.main" />
                Error: {attempt.statusText}
              </Text>
            )}
            <Box mt={4}>
              <ButtonPrimary
                onClick={() => handleSubmit(validator)}
                disabled={attempt.status === 'processing'}
              >
                Next
              </ButtonPrimary>
              <ButtonSecondary
                ml={3}
                onClick={() => props.onPrev({ ...props.awsOidc, thumbprint })}
                disabled={attempt.status === 'processing'}
              >
                Back
              </ButtonSecondary>
            </Box>
          </>
        )}
      </Validation>
    </InstructionsContainer>
  );
}
