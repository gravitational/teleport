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

import { keyBy, at, map } from 'lodash';

export function makeNodes(jsonArray){
  const nodes = map(jsonArray, makeNode);
  return keyBy(nodes, 'advertiseIp');
}

export default function makeNode(json){
  const [ name ] = at(json, 'metadata.name');
  const [ labels ] = at(json, 'metadata.labels');
  const [ advertiseIp ] = at(labels, "gravitational.io/advertise-ip");
  const [ cpu ] = at(json, 'status.capacity.cpu');
  const [ memory ] = at(json, 'status.capacity.memory');
  const [ osImage ] = at(json, 'status.nodeInfo.osImage');
  return {
    labels: labels || [],
    advertiseIp,
    cpu,
    memory,
    osImage,
    name,
    details: json,
  }
}
