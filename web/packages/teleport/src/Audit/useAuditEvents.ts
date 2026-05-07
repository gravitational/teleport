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
import { Event, EventCode, formatters } from 'teleport/services/audit';
import Ctx from 'teleport/teleportContext';

export default function useAuditEvents(
  ctx: Ctx,
  clusterId: string,
  eventCode?: EventCode
) {
  const rangeOptions = useMemo(() => getRangeOptions(), []);
  const [range, setRange] = useState<EventRange>(rangeOptions[0]);
  const { attempt, setAttempt, run } = useAttempt('processing');
  const [results, setResults] = useState<EventResult>({
    events: [],
    fetchStartKey: '',
    fetchStatus: '',
  });
  const filterBy = eventCode ? formatters[eventCode].type : '';

  useEffect(() => {
    fetch();
  }, [clusterId, range]);

  // fetchMore gets events from last position from
  // last fetch, indicated by startKey. The response is
  // appended to existing events list.
  function fetchMore() {
    setResults({
      ...results,
      fetchStatus: 'loading',
    });
    ctx.auditService
      .fetchEvents(clusterId, {
        ...range,
        filterBy,
        startKey: results.fetchStartKey,
      })
      .then(res =>
        setResults({
          events: [...results.events, ...res.events],
          fetchStartKey: res.startKey,
          fetchStatus: res.startKey ? '' : 'disabled',
        })
      )
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }

  // fetch gets events from beginning of range and
  // replaces existing events list.
  function fetch() {
    run(() =>
      ctx.auditService
        .fetchEvents(clusterId, {
          ...range,
          filterBy,
        })
        .then(res =>
          setResults({
            events: res.events,
            fetchStartKey: res.startKey,
            fetchStatus: res.startKey ? '' : 'disabled',
          })
        )
    );
  }

  return {
    ...results,
    fetchMore,
    clusterId,
    attempt,
    range,
    setRange,
    rangeOptions,
    ctx,
  };
}

type EventResult = {
  events: Event[];
  fetchStatus: 'loading' | 'disabled' | '';
  fetchStartKey: string;
};

export type State = ReturnType<typeof useAuditEvents>;
