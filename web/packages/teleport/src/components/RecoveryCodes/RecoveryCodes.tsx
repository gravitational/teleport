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

import React, { useRef } from 'react';
import styled from 'styled-components';
import { Box, ButtonPrimary, Card, Flex, Text } from 'design';
import copyToClipboard from 'design/utils/copyToClipboard';
import selectElementContent from 'design/utils/selectElementContent';

import { RecoveryCodes } from 'teleport/services/auth';
import { CaptureEvent, userEventService } from 'teleport/services/userEvent';

export default function RecoveryCodesDialog({
  recoveryCodes,
  onContinue,
  isNewCodes,
  continueText = 'Continue',
  username = '',
}: Props) {
  const codesRef = useRef();

  const captureRecoveryCodeEvent = (event: CaptureEvent) => {
    if (username) {
      userEventService.capturePreUserEvent({
        event: event,
        username: username,
      });
    }
  };

  const onCopyClick = () => {
    copyToClipboard(
      `${recoveryCodes?.codes.join('\n')} \n\nCreated: ${
        recoveryCodes?.createdDate
      }`
    ).then(() => {
      selectElementContent(codesRef.current);
    });
    captureRecoveryCodeEvent(CaptureEvent.PreUserRecoveryCodesCopyClickEvent);
  };

  const onPrintClick = () => {
    window.print();
    captureRecoveryCodeEvent(CaptureEvent.PreUserRecoveryCodesPrintClickEvent);
  };

  const handleContinue = () => {
    captureRecoveryCodeEvent(
      CaptureEvent.PreUserRecoveryCodesContinueClickEvent
    );
    onContinue();
  };

  let title = 'Backup & Recovery Codes';
  let btnText = `I have saved my Recovery Codes - ${continueText}`;
  if (isNewCodes) {
    title = 'New Backup & Recovery Codes';
    btnText = `I have saved my new Recovery Codes - ${continueText}`;
  }

  return (
    <PrintWrapper>
      <Card
        as={Flex}
        flexWrap="wrap"
        mx="auto"
        minWidth="584px"
        maxWidth="1024px"
        borderRadius={8}
        overflow="hidden"
        className="no-print"
      >
        <Flex
          flex={4}
          bg="levels.surface"
          minWidth="584px"
          flexDirection="column"
          p={5}
          className="print"
        >
          <Box mb={5}>
            <Text typography="h2" mb={3} color="light">
              {title}
            </Text>
            <Text mb={1}>
              Please save these account recovery codes in a safe offline place.
            </Text>
            <Text>
              You can use each code once if you lose your second factor
              authenticator or password.
            </Text>
          </Box>
          <Box>
            <Text bold mb={2} caps>
              Recovery Codes ({recoveryCodes?.codes.length} Total)
            </Text>
            <Flex
              bg="levels.sunkenSecondary"
              p={2}
              pb={4}
              pl={3}
              borderRadius={8}
              justifyContent="space-between"
            >
              <Text
                style={{ whiteSpace: 'pre-wrap' }}
                mt={2}
                ref={codesRef}
                className="codes"
              >
                {recoveryCodes?.codes.join('\n\n')}
              </Text>
              <Flex flexDirection="column" className="no-print" ml={2}>
                <MiniActionButton onClick={onCopyClick}>COPY</MiniActionButton>
                <MiniActionButton onClick={onPrintClick} mt={2}>
                  PRINT
                </MiniActionButton>
              </Flex>
            </Flex>
            <Text className="print-only">
              {`Created: ${recoveryCodes?.createdDate.toString()}`}
            </Text>
            <ButtonPrimary
              mt={6}
              size="large"
              width="100%"
              className="no-print"
              onClick={handleContinue}
            >
              {btnText}
            </ButtonPrimary>
          </Box>
        </Flex>
        <Flex
          flex={2}
          bg="levels.surfaceSecondary"
          minWidth="384px"
          flexDirection="column"
          p={5}
        >
          <Box mb={4}>
            <Text typography="h4" mb={2}>
              Why do I need these codes?
            </Text>
            <Text color="text.secondary">
              Use them in the event of losing your password or two-factor
              device.
            </Text>
          </Box>
          <Box mb={4}>
            <Text typography="h4" mb={2}>
              How long do the codes last for?
            </Text>
            <Text color="text.secondary">
              Recovery codes can only be used once. After recovering your
              account, we will generate a new set of codes for you.
            </Text>
          </Box>
          {isNewCodes && (
            <Box>
              <Text typography="h4" mb={2}>
                What about my old codes?
              </Text>
              <Text color="text.secondary">
                Your old recovery codes are no longer valid, please replace them
                with these new ones.
              </Text>
            </Box>
          )}
        </Flex>
      </Card>
    </PrintWrapper>
  );
}

const PrintWrapper = styled(Box)`
  .print-only {
    visibility: hidden;
  }

  @media print {
    overflow: hidden;
    .print,
    .print-only {
      visibility: visible;
    }
    .no-print {
      visibility: hidden;
    }
    .codes {
      font-size: 16px;
    }
  }
`;

const MiniActionButton = styled(ButtonPrimary)`
  max-width: 48px;
  width: 100%;
  padding: 4px 8px;
  min-height: 10px;
  font-size: 10px;
`;

export type Props = {
  recoveryCodes: RecoveryCodes;
  onContinue: () => void;
  isNewCodes: boolean;
  continueText?: string;
  username?: string;
};
