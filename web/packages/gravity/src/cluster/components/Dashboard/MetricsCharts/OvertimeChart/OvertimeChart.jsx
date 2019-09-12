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
import LineChart from './LineChart';

export default function UsageOverTime({ metrics, ...styles }){
  // reference to chart component
  const chartRef = React.useRef();
  // ref to current props.metrics value to access to latest inside setInterval callback
  const metricsRef = React.useRef(metrics);
  // flag to trigger the refresh interval
  const [ initialized, setInitialized ] = React.useState(false);

  const hasData = metrics.cpu.data.length > 0 || metrics.ram.data.length > 0;

  React.useEffect(() => {
    // update reference to metrics
    metricsRef.current = metrics;
  });

  // initialize the chart when data is ready
  React.useEffect(() => {
    const { cpu, ram } = makeSeries(metrics);
    chartRef.current.init(cpu, ram);
    setInitialized(true);
  }, [hasData])

  // start refreshing the chart
  React.useEffect(() => {
    if(!initialized){
      return;
    }

    function updateChart(){
      const { cpu, ram } = makeSeries(metricsRef.current);
      chartRef.current.add(cpu.pop(), ram.pop())
    }

    const timerId = setInterval(updateChart, 3000);

    return function cleanup() {
      clearInterval(timerId)
    };

  }, [initialized]);


  return (
    <LineChart ref={chartRef} {...styles} />
  )
}

function makeSeries(metrics){
  const cpu = metrics.cpu.data.map(r => r.value);
  const ram = metrics.ram.data.map(r => r.value);
  return {
    cpu,
    ram
  }
}
