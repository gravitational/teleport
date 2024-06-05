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

import React, { useState } from 'react';

import { SlideTabs } from './SlideTabs';

export default {
  title: 'Design/SlideTabs',
};

export const ThreeTabs = () => {
  const [activeIndex, setActiveIndex] = useState(0);
  return (
    <SlideTabs
      tabs={['aws', 'automatically', 'manually']}
      onChange={setActiveIndex}
      activeIndex={activeIndex}
    />
  );
};

export const FiveTabs = () => {
  const [activeIndex, setActiveIndex] = useState(0);
  return (
    <SlideTabs
      tabs={['step1', 'step2', 'step3', 'step4', 'step5']}
      onChange={setActiveIndex}
      activeIndex={activeIndex}
    />
  );
};

export const Round = () => {
  const [activeIndex, setActiveIndex] = useState(0);
  return (
    <SlideTabs
      appearance="round"
      tabs={['step1', 'step2', 'step3', 'step4', 'step5']}
      onChange={setActiveIndex}
      activeIndex={activeIndex}
    />
  );
};

export const Medium = () => {
  const [activeIndex, setActiveIndex] = useState(0);
  return (
    <SlideTabs
      tabs={['step1', 'step2', 'step3', 'step4', 'step5']}
      size="medium"
      appearance="round"
      onChange={setActiveIndex}
      activeIndex={activeIndex}
    />
  );
};

export const LoadingTab = () => {
  return (
    <SlideTabs
      tabs={['aws', 'automatically', 'manually']}
      onChange={() => null}
      activeIndex={1}
      isProcessing={true}
    />
  );
};
