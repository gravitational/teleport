import React, { useRef, useState } from 'react';
import styled from 'styled-components';
import { Box, ButtonIcon, ButtonPrimary, Card, Flex, H2, Text } from 'design';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import selectElementContent from 'design/utils/selectElementContent';

import { CaptureEvent, userEventService } from 'teleport/services/userEvent';
import * as Icon from 'design/Icon';
import { RecoveryCodes as RecoveryCodesData } from 'teleport/services/auth';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';

export type RecoveryCodesProps = {
  recoveryCodes: RecoveryCodesData;
  onContinue: () => void;
  isNewCodes: boolean;
  continueText?: string;
  username?: string;
};

export function RecoveryCodes({
  recoveryCodes,
  onContinue,
  isNewCodes,
  continueText = 'Continue',
  username = '',
}: RecoveryCodesProps) {
  const codesRef = useRef();
  const [codesSaved, setCodesSaved] = useState(false);

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
      `${recoveryCodes.codes.join('\n')} \n\nCreated: ${
        recoveryCodes.createdDate
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

  const onSavedClick = (e: React.ChangeEvent<HTMLInputElement>) => {
    setCodesSaved(e.target.checked);
  };

  const handleContinue = () => {
    captureRecoveryCodeEvent(
      CaptureEvent.PreUserRecoveryCodesContinueClickEvent
    );
    onContinue();
  };

  let title = 'Backup & Recovery Codes';
  let checkboxText = `I have saved my recovery codes`;
  if (isNewCodes) {
    title = 'New Backup & Recovery Codes';
    checkboxText = `I have saved my new recovery codes`;
  }

  return (
    <PrintWrapper>
      <Flex mx="auto" flexDirection="column" gap={3} width="750px">
        <Card flex={4} className="no-print-border" p={4}>
          <H2 mb={4}>{title}</H2>
          <Flex flexDirection="column" gap={3}>
            <Box>
              <Text>
                Save these account recovery codes in a safe offline place. You
                can use each code once if you lose your second factor
                authenticator or password.
              </Text>
            </Box>
            <Box>
              <Text mb={1}>
                Recovery Codes ({recoveryCodes.codes.length} Total)
              </Text>
              <Flex
                bg="levels.deep"
                p={3}
                gap={2}
                borderRadius={2}
                justifyContent="space-between"
              >
                <Text
                  style={{ whiteSpace: 'pre-wrap' }}
                  ref={codesRef}
                  className="codes"
                >
                  {recoveryCodes.codes.join('\n\n')}
                </Text>
                <Flex flexDirection="row" gap={2} className="no-print">
                  <ButtonIcon size={2} onClick={onCopyClick}>
                    <Icon.Copy />
                  </ButtonIcon>
                  <ButtonIcon size={2} onClick={onPrintClick}>
                    <Icon.Printer />
                  </ButtonIcon>
                </Flex>
              </Flex>
            </Box>
            <Text className="print-only">
              {`Created: ${recoveryCodes.createdDate.toString()}`}
            </Text>
            <FieldCheckbox
              label={checkboxText}
              checked={codesSaved}
              onChange={onSavedClick}
              mb={0}
            />
            <ButtonPrimary
              size="large"
              width="100%"
              className="no-print"
              disabled={!codesSaved}
              onClick={handleContinue}
            >
              {continueText}
            </ButtonPrimary>
          </Flex>
        </Card>
        <Card flex={2} minWidth="384px" className="no-print" p={4}>
          <Flex flexDirection="column" gap={3}>
            <Box>
              <Text typography="h5">1. Why do I need these codes?</Text>
              <Text color="text.slightlyMuted">
                Use them in the event of losing your password or two-factor
                device.
              </Text>
            </Box>
            <Box>
              <Text typography="h5">2. How long do the codes last for?</Text>
              <Text color="text.slightlyMuted">
                Recovery codes can only be used once. After recovering your
                account, we will generate a new set of codes for you.
              </Text>
            </Box>
            {isNewCodes && (
              <Box>
                <Text typography="h5">3. What about my old codes?</Text>
                <Text color="text.slightlyMuted">
                  Your old recovery codes are no longer valid, please replace
                  them with these new ones.
                </Text>
              </Box>
            )}
          </Flex>
        </Card>
      </Flex>
    </PrintWrapper>
  );
}

const PrintWrapper = styled(Box)`
  .print-only {
    display: none;
  }

  @media print {
    .print-only {
      display: initial;
    }
    .no-print {
      display: none;
    }
    .no-print-border {
      border: none;
      box-shadow: none;
    }
    .codes {
      font-size: 16px;
    }
  }
`;
