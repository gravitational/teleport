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
import { Flex, LabelInput, Text } from 'design';

import Select, { Option } from 'shared/components/Select';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { AccessRequest } from 'shared/services/accessRequests';

import { getDurationOptionsFromStartTime } from './durationOptions';

export function AccessDurationRequest({
  assumeStartTime,
  accessRequest,
  maxDuration,
  setMaxDuration,
}: {
  assumeStartTime: Date;
  accessRequest: AccessRequest;
  maxDuration: Option<number>;
  setMaxDuration(s: Option<number>): void;
}) {
  // Options for extending or shortening the access request duration.
  const durationOptions = getDurationOptionsFromStartTime(
    assumeStartTime,
    accessRequest
  );

  return (
    <LabelInput typography="body2" color="text.slightlyMuted">
      <Flex alignItems="center">
        <Text mr={1}>Access Duration</Text>
        <ToolTipInfo>
          How long you would be given elevated privileges. Note that the time it
          takes to approve this request will be subtracted from the duration you
          requested.
        </ToolTipInfo>
      </Flex>

      <Select
        options={durationOptions}
        onChange={setMaxDuration}
        value={maxDuration}
      />
    </LabelInput>
  );
}
