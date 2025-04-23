/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useEffect, useMemo, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import {
  EventRange,
  getRangeOptions,
} from 'teleport/components/EventRangePicker';
import { Recording } from 'teleport/services/recordings';
import Ctx from 'teleport/teleportContext';
import useStickyClusterId from 'teleport/useStickyClusterId';

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
    ctx,
    attempt,
    range,
    rangeOptions,
    setRange,
    clusterId,
    fetchMore,
  };
}

type RecordingsResult = {
  recordings: Recording[];
  fetchStatus: 'loading' | 'disabled' | '';
  fetchStartKey: string;
};

export type State = ReturnType<typeof useRecordings>;
