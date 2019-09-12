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

import { at } from 'lodash';

export default function makeNodeProfile(json) {
  const [
    ram,
    cpuCount,
    expandPolicy,
    description,
    serviceRole,
    name,
  ] = at(json,
    [
      'requirements.ram.min',
      'requirements.cpu.min',
      'expandPolicy',
      'description',
      'serviceRole',
      'name'
    ]);

  const requirementsText = `RAM: ${ram}, CPU: Core x ${cpuCount}`;

  return {
    ram,
    name,
    cpuCount,
    serviceRole,
    description,
    expandPolicy,
    requirementsText
  }
}