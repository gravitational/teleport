import React, { useRef } from 'react';
import { Box, Flex, Text, ButtonPrimary } from 'design';
import { pluralize } from 'teleport/lib/util';
import { AssumedRequest } from 'teleterm/services/tshd/types';

import { useAssumedRolesBar } from './useAssumedRolesBar';

export function AssumedRolesBar({ assumedRolesRequest }: Props) {
  const {
    duration,
    assumedRoles,
    dropRequest,
    dropRequestAttempt,
    hasExpired,
  } = useAssumedRolesBar(assumedRolesRequest);
  const roleText = pluralize(assumedRoles.length, 'role');
  const durationText = `${roleText} assumed, expires in ${duration}`;
  const hasExpiredText =
    assumedRoles.length > 1 ? 'have expired' : 'has expired';
  const expirationText = `${roleText} ${hasExpiredText}`;
  const ref = useRef<HTMLButtonElement>(null);
  const assumedRolesText = assumedRoles.join(', ');
  return (
    <Box px={3} py={2} bg="accent" borderTop={1} borderColor="primary.dark">
      <Flex justifyContent="space-between" alignItems="center">
        <Flex alignItems="center">
          <Box
            borderRadius="20px"
            py={1}
            px={3}
            mr={2}
            color="secondary.main"
            bg="light"
            style={{
              fontWeight: '500',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              maxWidth: '200px',
              whiteSpace: 'nowrap',
            }}
            title={assumedRolesText}
          >
            {assumedRolesText}
          </Box>
          <Text typography="body" color="light" bold>
            {hasExpired ? expirationText : durationText}
          </Text>
        </Flex>
        <ButtonPrimary
          setRef={ref}
          onClick={dropRequest}
          disabled={dropRequestAttempt.status === 'processing'}
        >
          Drop Request
        </ButtonPrimary>
      </Flex>
    </Box>
  );
}

type Props = {
  assumedRolesRequest: AssumedRequest;
};
