import React from 'react';
import Flex from 'design/Flex';
import { Info } from 'design/Icon';
import Text from 'design/Text';

import type { Severity } from 'e-teleport/services/license';

export function LicenseWarning({ text, severity }: Props) {
  return (
    <Flex
      height="48px"
      bg={getWarningColor(severity)}
      justifyContent="center"
      data-testid="warning"
    >
      <Flex width="100%" justifyContent="center" alignItems="center" px="5">
        <Flex alignItems="center">
          <Info mr={2} fontSize="3" />
          <Text bold textAlign="center">
            {text}
          </Text>
        </Flex>
      </Flex>
    </Flex>
  );
}

function getWarningColor(severity: Severity): string {
  switch (severity) {
    case 'info':
      return 'info';
    case 'error':
      return 'danger';
    case 'warning':
      return 'warning';
    default:
      return 'secondary.light';
  }
}

export type Props = {
  text: string;
  severity: Severity;
};
