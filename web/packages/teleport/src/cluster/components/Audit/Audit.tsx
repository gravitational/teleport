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

import React, { useEffect, useState, useMemo } from 'react';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import AuditEvents from './AuditEvents';
import RangePicker, { getRangeOptions } from './RangePicker';
import { useAttempt } from 'shared/hooks';
import { Danger } from 'design/Alert';
import { Indicator, Box } from 'design';
import { useTeleport } from 'teleport/teleportContextProvider';

export default function Audit() {
  const rangeOptions = useMemo(() => getRangeOptions(), []);
  const [range, handleOnRange] = useState(rangeOptions[0]);

  const { overflow, attempt, maxLimit, events } = useEvents(range);
  const { isSuccess, isFailed, message, isProcessing } = attempt;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle mr="8">Audit Log</FeatureHeaderTitle>
        <RangePicker
          ml="auto"
          value={range}
          options={rangeOptions}
          onChange={handleOnRange}
        />
      </FeatureHeader>
      {overflow && (
        <Danger>
          Number of events retrieved for specified date range has exceeded the
          maximum limit of {maxLimit} events
        </Danger>
      )}
      {isFailed && <Danger> {message} </Danger>}
      {isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {isSuccess && <AuditEvents events={events} />}
    </FeatureBox>
  );
}

function useEvents(range: Range) {
  const teleCtx = useTeleport();
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [results, setResults] = useState({
    events: [],
    overflow: false,
  });

  function onFetch({ from, to }: Range) {
    attemptActions.do(() => {
      return teleCtx.auditService
        .fetchEvents({ start: from, end: to })
        .then(setResults);
    });
  }

  function onFetchLatest() {
    return teleCtx.auditService.fetchLatest().then(setResults);
  }

  useEffect(() => {
    onFetch(range);
  }, [range]);

  return {
    ...results,
    attempt,
    attemptActions,
    onFetch,
    onFetchLatest,
    maxLimit: teleCtx.auditService.maxLimit,
  };
}

type Range = { from: Date; to: Date };
