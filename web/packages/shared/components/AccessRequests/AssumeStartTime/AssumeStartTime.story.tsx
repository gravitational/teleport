/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useState } from 'react';
import { format } from 'date-fns';

import { Box, Text } from 'design';

import { Option } from 'shared/components/Select';
import cfg from 'shared/config';

import { dryRunResponse } from '../fixtures';
import { AccessDurationRequest, AccessDurationReview } from '../AccessDuration';

import { AssumeStartTime } from './AssumeStartTime';

export default {
  title: 'Shared/AccessRequests/AssumeStartTime',
};

export const NewRequest = () => {
  const [start, setStart] = useState<Date>();
  const [maxDuration, setMaxDuration] = useState<Option<number>>();

  return (
    <Box width="400px">
      <Box mb={4}>
        <Text>Sample Dry Run Access Requeset Response:</Text>
        <Text>
          <b>Created Date:</b>{' '}
          {format(dryRunResponse.created, cfg.dateTimeFormat)}
        </Text>
        <Text>
          <b>Max Duration Date:</b>{' '}
          {format(dryRunResponse.maxDuration, cfg.dateTimeFormat)}
        </Text>
      </Box>
      <AssumeStartTime
        start={start}
        onStartChange={setStart}
        accessRequest={dryRunResponse}
      />
      <AccessDurationRequest
        maxDuration={maxDuration}
        onMaxDurationChange={setMaxDuration}
        maxDurationOptions={[]}
      />
    </Box>
  );
};

export const CreatedRequestWithoutStart = () => {
  const [start, setStart] = useState<Date>();

  return (
    <Box width="400px">
      <Box mb={4}>
        <Text>Sample Access Request:</Text>
        <Text>
          <b>Created Date:</b>{' '}
          {format(dryRunResponse.created, cfg.dateTimeFormat)}
        </Text>
        <Text>
          <b>Max Duration Date:</b>{' '}
          {format(dryRunResponse.maxDuration, cfg.dateTimeFormat)}
        </Text>
      </Box>
      <AssumeStartTime
        start={start}
        onStartChange={setStart}
        accessRequest={dryRunResponse}
        reviewing={true}
      />
      <AccessDurationReview
        assumeStartTime={start}
        accessRequest={dryRunResponse}
      />
    </Box>
  );
};

export const CreatedRequestWithStart = () => {
  const [start, setStart] = useState<Date>();

  const withStart = {
    ...dryRunResponse,
    assumeStartTime: new Date('2024-02-14T02:51:12.70087Z'),
  };

  return (
    <Box width="400px">
      <Box mb={4}>
        <Text>Sample Access Request:</Text>
        <Text>
          <b>Created Date:</b> {format(withStart.created, cfg.dateTimeFormat)}
        </Text>
        <Text>
          <b>Max Duration Date:</b>{' '}
          {format(withStart.maxDuration, cfg.dateTimeFormat)}
        </Text>
      </Box>
      <AssumeStartTime
        start={start}
        onStartChange={setStart}
        accessRequest={withStart}
        reviewing={true}
      />
      <AccessDurationReview
        assumeStartTime={start}
        accessRequest={dryRunResponse}
      />
    </Box>
  );
};
