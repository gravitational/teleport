/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import SlideTabs from './SlideTabs';

export default {
  title: 'Design/SlideTabs',
};

export const ThreeTabs = () => {
  return (
    <SlideTabs
      tabs={['aws', 'automatically', 'manually']}
      onChange={() => {}}
    />
  );
};

export const FiveTabs = () => {
  return (
    <SlideTabs
      tabs={['step1', 'step2', 'step3', 'step4', 'step5']}
      onChange={() => {}}
    />
  );
};

export const Round = () => {
  return (
    <SlideTabs
      appearance="round"
      tabs={['step1', 'step2', 'step3', 'step4', 'step5']}
      onChange={() => {}}
    />
  );
};

export const Medium = () => {
  return (
    <SlideTabs
      tabs={['step1', 'step2', 'step3', 'step4', 'step5']}
      size="medium"
      appearance="round"
      onChange={() => {}}
    />
  );
};
