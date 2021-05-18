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
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import EventList from './EventList';
import RangePicker from 'teleport/components/EventRangePicker';
import { Danger } from 'design/Alert';
import { Flex, Indicator, Box } from 'design';
import InputSearch from 'teleport/components/InputSearch';
import useTeleport from 'teleport/useTeleport';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useAuditEvents from 'teleport/useAuditEvents';

export default function Container() {
  const teleCtx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const state = useAuditEvents(teleCtx, clusterId);
  return <Audit {...state} />;
}

export function Audit(props: ReturnType<typeof useAuditEvents>) {
  const {
    overflow,
    attempt,
    maxLimit,
    range,
    rangeOptions,
    setRange,
    events,
    searchValue,
    clusterId,
    setSearchValue,
  } = props;

  const { isSuccess, isFailed, message, isProcessing } = attempt;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Audit Log</FeatureHeaderTitle>
        <RangePicker
          ml="auto"
          value={range}
          options={rangeOptions}
          onChange={setRange}
        />
      </FeatureHeader>
      <Flex
        mb={4}
        alignItems="center"
        flex="0 0 auto"
        justifyContent="flex-start"
      >
        <InputSearch mr="3" onChange={setSearchValue} />
      </Flex>
      {overflow && (
        <Danger>
          number of events retrieved for specified date range has exceeded the
          maximum limit of {maxLimit} events
        </Danger>
      )}
      {isFailed && <Danger> {message} </Danger>}
      {isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {isSuccess && (
        <EventList search={searchValue} events={events} clusterId={clusterId} />
      )}
    </FeatureBox>
  );
}
