/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useState, useEffect, useMemo } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import Ctx from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';
import {
  getRangeOptions,
  EventRange,
} from 'teleport/components/EventRangePicker';
import { Recording } from 'teleport/services/recordings';

export default function useRecordings(ctx: Ctx) {
  const { clusterId } = useStickyClusterId();
  const rangeOptions = useMemo(() => getRangeOptions(), []);
  const [range, setRange] = useState<EventRange>(rangeOptions[0]);
  const { attempt, setAttempt, run } = useAttempt('processing');
  const [results, setResults] = useState<RecordingsResult>({
    recordings: [],
    fetchStartKey: '',
    fetchStatus: '',
  });
  const showByobCta = ctx.isEnterprise; // TODO: and not byob user yet

  function fetchMore() {
    setResults({
      ...results,
      fetchStatus: 'loading',
    });
    ctx.recordingsService
      .fetchRecordings(clusterId, {
        ...range,
        startKey: results.fetchStartKey,
      })
      .then(res =>
        setResults({
          recordings: [...results.recordings, ...res.recordings],
          fetchStartKey: res.startKey,
          fetchStatus: res.startKey ? '' : 'disabled',
        })
      )
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  function fetch() {
    run(() =>
      ctx.recordingsService
        .fetchRecordings(clusterId, {
          ...range,
        })
        .then(res =>
          setResults({
            recordings: res.recordings,
            fetchStartKey: res.startKey,
            fetchStatus: res.startKey ? '' : 'disabled',
          })
        )
    );
  }

  useEffect(() => {
    fetch();
  }, [clusterId, range]);

  return {
    ...results,
    attempt,
    range,
    rangeOptions,
    setRange,
    clusterId,
    fetchMore,
    showByobCta,
  };
}

type RecordingsResult = {
  recordings: Recording[];
  fetchStatus: 'loading' | 'disabled' | '';
  fetchStartKey: string;
};

export type State = ReturnType<typeof useRecordings>;
