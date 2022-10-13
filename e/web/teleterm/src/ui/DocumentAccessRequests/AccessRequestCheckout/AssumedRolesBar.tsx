import React, { useRef } from 'react';

import { Box, Flex, Text, ButtonPrimary } from 'design';

import { pluralize } from 'teleport/lib/util';
import { getDurationText } from 'shared/utils/getDurationText';
import { AssumedRequest } from 'teleterm/services/tshd/types';

import useAssumedRolesBar from './useAssumedRolesbar';

export function AssumedRolesBar({ assumedRolesRequest }: Props) {
  const { time, assumedRoles, switchBack } =
    useAssumedRolesBar(assumedRolesRequest);
  const durationTxt = getDurationText(time.hours, time.minutes, time.seconds);
  const ref = useRef<HTMLButtonElement>(null);
  const roles = assumedRoles.join(', ');
  const roleText = pluralize(assumedRoles.length, 'role');
  return (
    <Box px={3} py={2} bg="accent" border={1} borderColor="primary.dark">
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
            title={roles}
          >
            {roles}
          </Box>
          <Text typography="body" color="light" bold>
            {roleText}, expires in {durationTxt}
          </Text>
        </Flex>
        <ButtonPrimary setRef={ref} onClick={switchBack}>
          Switch Back
        </ButtonPrimary>
      </Flex>
    </Box>
  );
}

type Props = {
  assumedRolesRequest: AssumedRequest;
};
