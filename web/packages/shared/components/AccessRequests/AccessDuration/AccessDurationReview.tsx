/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import React from 'react';
import { Flex, Text } from 'design';

import { ToolTipInfo } from 'shared/components/ToolTip';

import { AccessRequest } from 'shared/services/accessRequests';

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
