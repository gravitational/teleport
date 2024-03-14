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
import { Option } from 'shared/components/Select';
import { Box, Text } from 'design';

import { dryRunResponse } from '../fixtures';
import { Start } from '../Shared/types';

import { AssumeStartTime } from './AssumeStartTime';

export default {
  title: 'TeleportE/Workflow/AssumeStartTime',
};

export const NewRequest = () => {
  const [start, setStart] = useState<Start>();
  const [maxDuration, setMaxDuration] = useState<Option<number>>();

  return (
    <Box width="400px">
      <Box mb={4}>
        <Text>Sample Dry Run Access Requeset Response:</Text>
        <Text>
          <b>Created Date:</b>{' '}
          {format(dryRunResponse.created, 'yyyy-MM-dd HH:mm:ss')}
        </Text>
        <Text>
          <b>Max Duration Date:</b>{' '}
          {format(dryRunResponse.maxDuration, 'yyyy-MM-dd HH:mm:ss')}
        </Text>
      </Box>
      <AssumeStartTime
        start={start}
        setStart={setStart}
        accessRequest={dryRunResponse}
        maxDuration={maxDuration}
        setMaxDuration={setMaxDuration}
      />
    </Box>
  );
};
