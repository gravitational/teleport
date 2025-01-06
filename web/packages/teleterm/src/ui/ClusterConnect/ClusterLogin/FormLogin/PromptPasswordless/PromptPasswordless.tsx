/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex, Image, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';

import { LinearProgress } from 'teleterm/ui/components/LinearProgress';

import type { PasswordlessLoginState } from '../../useClusterLogin';
import svgHardwareKey from './hardware.svg';

type Props = PasswordlessLoginState & {
  onCancel(): void;
};

export function PromptPasswordless(props: Props) {
  const { prompt } = props;
  return (
    <Box minHeight="40px">
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
      <UsernamesContainer disabled={processing}>
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
                  background: ${props => props.theme.colors.spotBackground[0]};
                }
              `}
              onClick={() => onUserResponse(index)}
            >
              {username}
            </button>
          );
        })}
      </UsernamesContainer>
      <ActionButtons onCancel={onCancel} />
    </Box>
  );
}

const UsernamesContainer = styled(Box)<{ disabled?: boolean }>`
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
`;

function PromptPin({ onCancel, onUserResponse, processing }: Props) {
  const [pin, setPin] = useState('');

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
    <Flex justifyContent="flex-start" mt={4}>
      {/*
        Generally, every other modal in the app with a "Cancel" button has the button to the right
        of the button like "Next".

        However, when using a hardware key with a PIN, the user goes through a series of steps where
        the "Cancel" key is always present. The "Next" button is present only when entering the PIN,
        so it makes sense to show it to the right of the "Cancel" button.
      */}
      <ButtonSecondary
        type="button"
        onClick={onCancel}
        mr={nextButton.isVisible ? 3 : 0}
      >
        Cancel
      </ButtonSecondary>
      {/* The caller of this component needs to handle wrapping
      this in a <form> element to handle `onSubmit` event on enter key*/}
      {nextButton.isVisible && (
        <ButtonPrimary type="submit" disabled={nextButton.isDisabled}>
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
      message: 'PIN must be at least 4 characters',
    };
  }

  return {
    valid: true,
  };
};
