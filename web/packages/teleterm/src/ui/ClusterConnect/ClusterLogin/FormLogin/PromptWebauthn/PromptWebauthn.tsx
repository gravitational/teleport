/**
 * Copyright 2021-2022 Gravitational, Inc.
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

import React from 'react';
import { Box, ButtonSecondary, ButtonPrimary, Text, Image, Flex } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

import svgHardwareKey from './hardware.svg';

import type { WebauthnLogin } from '../../useClusterLogin';

export function PromptWebauthn(props: Props) {
  const { prompt } = props;
  return (
    <Box minHeight="40px" p={4}>
      {prompt === 'credential' && <PromptCredential {...props} />}
      {(prompt === 'tap' || prompt === 'retap') && <PromptTouch {...props} />}
      {prompt === 'pin' && <PromptPin {...props} />}
    </Box>
  );
}

function PromptTouch({ onCancel, prompt }: Props) {
  return (
    <>
      <Image mb={4} width="200px" src={svgHardwareKey} mx="auto" />
      <Box textAlign="center" style={{ position: 'relative' }}>
        {prompt === 'retap' ? (
          <Text bold>Tap your security key again to complete the request</Text>
        ) : (
          <Text bold>Insert your security key and tap it</Text>
        )}
        <LinearProgress />
      </Box>
      <ActionButtons onCancel={onCancel} />
    </>
  );
}

function PromptCredential({
  loginUsernames,
  onUserResponse,
  processing,
  onCancel,
}: Props) {
  return (
    <Box height="330px" width="100%">
      <Text mb={2} bold>
        Select the user for login:
      </Text>
      <Box
        disabled={processing}
        css={`
          overflow: auto;
          height: 240px;
          width: 100%;
          padding: 8px;
          border: 1px solid #252c52;
          border-radius: 4px;
          &[disabled] {
            pointer-events: none;
            opacity: 0.5;
          }
        `}
      >
        {loginUsernames.map((username, index) => {
          return (
            <button
              key={index}
              autoFocus={index === 0}
              css={`
                background: inherit;
                color: inherit;
                font-size: inherit;
                font-family: inherit;
                line-height: inherit;
                display: block;
                border: none;
                padding: 8px;
                width: 100%;
                text-align: left;
                &:hover,
                &:focus {
                  cursor: pointer;
                  background: ${props => props.theme.colors.action.hover};
                }
              `}
              onClick={() => onUserResponse(index)}
            >
              {username}
            </button>
          );
        })}
      </Box>
      <ActionButtons onCancel={onCancel} />
    </Box>
  );
}

function PromptPin({ onCancel, onUserResponse, processing }: Props) {
  const [pin, setPin] = React.useState('');

  return (
    <Validation>
      {({ validator }) => (
        <form
          onSubmit={e => {
            e.preventDefault();
            validator.validate() && onUserResponse(pin);
          }}
        >
          <Box>
            <FieldInput
              width="240px"
              label="Enter the PIN for your security key"
              rule={requiredLength}
              type="password"
              value={pin}
              onChange={e => setPin(e.target.value.trim())}
              placeholder="1234"
              autoFocus
            />
          </Box>
          <ActionButtons
            onCancel={onCancel}
            nextButton={{
              isVisible: true,
              isDisabled: processing || pin.length === 0,
            }}
          />
        </form>
      )}
    </Validation>
  );
}

function ActionButtons({
  onCancel,
  nextButton = { isVisible: false, isDisabled: false },
}: {
  onCancel(): void;
  nextButton?: {
    isVisible: boolean;
    isDisabled: boolean;
  };
}) {
  return (
    <Flex justifyContent="flex-end" mt={4}>
      <ButtonSecondary
        type="button"
        width={80}
        size="small"
        onClick={onCancel}
        mr={nextButton.isVisible ? 3 : 0}
      >
        Cancel
      </ButtonSecondary>
      {/* The caller of this component needs to handle wrapping
      this in a <form> element to handle `onSubmit` event on enter key*/}
      {nextButton.isVisible && (
        <ButtonPrimary
          type="submit"
          width={80}
          size="small"
          disabled={nextButton.isDisabled}
        >
          Next
        </ButtonPrimary>
      )}
    </Flex>
  );
}

const requiredLength = value => () => {
  if (!value || value.length < 4) {
    return {
      valid: false,
      message: 'pin must be at least 4 characters',
    };
  }

  return {
    valid: true,
  };
};

export type Props = WebauthnLogin & {
  onCancel(): void;
};
