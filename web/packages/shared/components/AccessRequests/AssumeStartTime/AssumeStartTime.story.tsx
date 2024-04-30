/**
 * Copyright 2023 Gravitational, Inc.
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
        assumeStartTime={start}
        accessRequest={dryRunResponse}
        maxDuration={maxDuration}
        setMaxDuration={setMaxDuration}
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
