import React from 'react';
import { Flex, Text } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { AccessRequest } from 'e-teleport/services/accessRequests';

import { getFormattedDurationTxt } from '../Shared/utils';

export function AccessDurationReview({
  assumeStartTime,
  accessRequest,
}: {
  assumeStartTime: Date;
  accessRequest: AccessRequest;
}) {
  return (
    <Flex alignItems="center">
      <Text mr={1}>
        <b>Access Duration: </b>
        {getFormattedDurationTxt({
          start: assumeStartTime || accessRequest.assumeStartTime || new Date(),
          end: accessRequest.expires,
        })}
      </Text>
      <ToolTipInfo>
        How long the access will be granted for after approval.
      </ToolTipInfo>
    </Flex>
  );
}
