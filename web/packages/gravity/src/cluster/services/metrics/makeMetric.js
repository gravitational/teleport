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

import { map } from 'lodash';

export default function makeMetrics(json){
  json = json || {};

  const {
    total_cpu_cores = 0,
    total_memory_bytes = 0,
    cpu_rates,
    memory_rates,
  } = json;

  return {
    cpuCoreTotal: total_cpu_cores,
    ramTotal: total_memory_bytes,
    cpu: makeMetric(cpu_rates),
    ram: makeMetric(memory_rates),
  }
}

function makeMetric(json){
  json = json || {};

  const { current=0, max=0, historic=[] } = json;
  const data = map(historic, h => ({
    time: new Date(h.time),
    value: h.value
  }))

  return {
    current,
    max,
    data,
  }
}