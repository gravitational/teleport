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
import { Flex } from 'design';
import { useFluxStore } from 'gravity/components/nuclear';
import AjaxPoller from 'gravity/components/AjaxPoller';
import { filesize } from 'gravity/lib/humanize';
import UsageOverTime from './OvertimeChart';
import CircleGraph from './CircleGraph';
import { fetchShortMetrics, fetchMetrics } from 'gravity/cluster/flux/metrics/actions';
import { getters } from 'gravity/cluster/flux/metrics';
import { useAttempt } from 'shared/hooks';

const POLL_INTERVAL = 5000; // every 5 sec

export default function MetricsCharts(props) {
  const { short, long } = useFluxStore(getters.metricsStore);
  const { cpu, cpuCoreTotal, ram, ramTotal } = short;
  const ramTotalText = filesize(ramTotal);
  const [ attempt, attemptActions ] = useAttempt();

  // fetch initial data which has long and short metric data
  React.useEffect(() => {
    attemptActions.do(() => {
      return fetchMetrics();
    });
  }, []);

  const ramSubtitles = [
    `Total ${ramTotalText}`,
    `High ${long.ram.max}%`
  ];

  const cpuSubtitles = [
    `${cpuCoreTotal} CPU Cores`,
    `High ${long.cpu.max}%`
  ];

  function onFetchMetrics(){
    return fetchShortMetrics();
  }

  const { isSuccess } = attempt;

  return (
    <Flex style={{flexShrink: "0"}} {...props}>
      <UsageOverTime flex="1" mb="4" mr="4" metrics={short}/>
      <CircleGraph mr="4" mb="4"
        title="CPU Usage"
        current={cpu.current}
        subtitles={cpuSubtitles}
      />
      <CircleGraph
        mb="4"
        title="RAM Usage"
        current={ram.current}
        subtitles={ramSubtitles}
      />
      { isSuccess && <AjaxPoller immediately={false} time={POLL_INTERVAL} onFetch={onFetchMetrics} />}
    </Flex>
  )
}