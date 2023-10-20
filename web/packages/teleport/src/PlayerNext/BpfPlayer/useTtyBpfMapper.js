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
import { throttle } from 'shared/utils/highbar';

export default function useTtyBpfMapper(tty, events) {
  // create a map [time][index] for quick lookups
  const bpfLookupTable = React.useMemo(() => {
    return events.map(i => new Date(i.time));
  }, [events]);

  // current cursor (index)
  const [cursor, setCursor] = React.useState(() => {
    return mapTtyToBpfEvent(tty, bpfLookupTable);
  });

  React.useEffect(() => {
    function onChange() {
      const index = mapTtyToBpfEvent(tty, bpfLookupTable);
      setCursor(index);
    }

    const throttledOnChange = throttle(onChange, 100);

    function cleanup() {
      throttledOnChange.cancel();
      tty.removeListener('change', throttledOnChange);
    }

    tty.on('change', throttledOnChange);

    return cleanup;
  }, [tty, events]);

  return cursor;
}

function mapTtyToBpfEvent(tty, bpfLookupTable = []) {
  if (bpfLookupTable.length === 0) {
    return -1;
  }

  // return the last event index if player exceeded
  // the total number of events
  if (tty.currentEventIndex >= tty._eventProvider.events.length) {
    return bpfLookupTable.length;
  }

  const ttyEvent = tty._eventProvider.events[tty.currentEventIndex];
  return getEventIndex(ttyEvent.time, bpfLookupTable);
}

// finds event index by datetime using binary search
function getEventIndex(datetime, lookupTable) {
  const arr = lookupTable;
  var low = 0;
  var hi = arr.length - 1;

  while (hi - low > 1) {
    const mid = Math.floor((low + hi) / 2);
    if (arr[mid] < datetime) {
      low = mid;
    } else {
      hi = mid;
    }
  }

  if (datetime - arr[low] <= arr[hi] - datetime) {
    return low;
  }

  return hi;
}
