/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';

import { Danger } from 'design/Alert';
import { Indicator, Box } from 'design';

import RangePicker from 'teleport/components/EventRangePicker';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useTeleport from 'teleport/useTeleport';

import RecordingsList from './RecordingsList';

import useRecordings, { State } from './useRecordings';

export default function Container() {
  const ctx = useTeleport();
  const state = useRecordings(ctx);
  return <Recordings {...state} />;
}

export function Recordings({
  recordings,
  fetchStatus,
  fetchMore,
  range,
  setRange,
  rangeOptions,
  attempt,
  clusterId,
}: State) {
  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Session Recordings</FeatureHeaderTitle>
        <RangePicker
          ml="auto"
          range={range}
          ranges={rangeOptions}
          onChangeRange={setRange}
        />
      </FeatureHeader>
      {attempt.status === 'failed' && <Danger> {attempt.statusText} </Danger>}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <RecordingsList
          recordings={recordings}
          clusterId={clusterId}
          fetchMore={fetchMore}
          fetchStatus={fetchStatus}
        />
      )}
    </FeatureBox>
  );
}
