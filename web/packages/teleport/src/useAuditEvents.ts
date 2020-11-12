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

import moment from 'moment';
import React, { useEffect, useState, useMemo } from 'react';
import { useAttempt } from 'shared/hooks';
import Ctx from 'teleport/teleportContext';

export default function useEvents(ctx: Ctx, clusterId: string) {
  const rangeOptions = useMemo(() => getRangeOptions(), []);
  const [searchValue, setSearchValue] = React.useState('');
  const [range, setRange] = useState(rangeOptions[1]);
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [results, setResults] = useState({
    events: [],
    overflow: false,
  });

  function onFetch({ from, to }: Range) {
    attemptActions.do(() => {
      return ctx.auditService
        .fetchEvents(clusterId, { start: from, end: to })
        .then(setResults);
    });
  }

  useEffect(() => {
    onFetch(range);
  }, [clusterId, range]);

  return {
    ...results,
    attempt,
    clusterId,
    attemptActions,
    onFetch,
    maxLimit: ctx.auditService.maxLimit,
    range,
    rangeOptions,
    setRange,
    searchValue,
    setSearchValue,
  };
}

export function getRangeOptions() {
  return [
    {
      name: 'Today',
      from: moment(new Date())
        .startOf('day')
        .toDate(),
      to: moment(new Date())
        .endOf('day')
        .toDate(),
    },
    {
      name: '7 days',
      from: moment()
        .subtract(6, 'day')
        .startOf('day')
        .toDate(),
      to: moment(new Date())
        .endOf('day')
        .toDate(),
    },
    {
      name: 'Custom Range...',
      isCustom: true,
      from: new Date(),
      to: new Date(),
    },
  ];
}

type Range = { from: Date; to: Date };
