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

import { useState } from 'react';

import Flex from 'design/Flex';
import * as Icon from 'design/Icon';

import { SlideTabs, TabSpec } from './SlideTabs';

export default {
  title: 'Design/SlideTabs',
};

const threeSimpleTabs = [
  { key: 'aws', title: 'aws' },
  { key: 'automatically', title: 'automatically' },
  { key: 'manually', title: 'manually' },
];

const fiveSimpleTabs = [
  { key: 'step1', title: 'step1' },
  { key: 'step2', title: 'step2' },
  { key: 'step3', title: 'step3' },
  { key: 'step4', title: 'step4' },
  { key: 'step5', title: 'step5' },
];

const titlesWithIcons = [
  { key: 'alarm', icon: Icon.AlarmRing, title: 'Clocks' },
  { key: 'bots', icon: Icon.Bots, title: 'Bots' },
  { key: 'check', icon: Icon.Check, title: 'Checkmarks' },
];

export const ThreeTabs = () => {
  const [activeIndex, setActiveIndex] = useState(0);
  return (
    <SlideTabs
      tabs={threeSimpleTabs}
      onChange={setActiveIndex}
      activeIndex={activeIndex}
    />
  );
};

export const FiveTabs = () => {
  const [activeIndex, setActiveIndex] = useState(0);
  return (
    <SlideTabs
      tabs={fiveSimpleTabs}
      onChange={setActiveIndex}
      activeIndex={activeIndex}
    />
  );
};

export const Round = () => {
  const [activeIndex1, setActiveIndex1] = useState(0);
  const [activeIndex2, setActiveIndex2] = useState(0);
  return (
    <Flex flexDirection="column" gap={3}>
      <SlideTabs
        appearance="round"
        tabs={fiveSimpleTabs}
        onChange={setActiveIndex1}
        activeIndex={activeIndex1}
      />
      <SlideTabs
        tabs={titlesWithIcons}
        appearance="round"
        onChange={setActiveIndex2}
        activeIndex={activeIndex2}
      />
    </Flex>
  );
};

export const Medium = () => {
  const [activeIndex1, setActiveIndex1] = useState(0);
  const [activeIndex2, setActiveIndex2] = useState(0);
  return (
    <Flex flexDirection="column" gap={3}>
      <SlideTabs
        tabs={fiveSimpleTabs}
        size="medium"
        appearance="round"
        onChange={setActiveIndex1}
        activeIndex={activeIndex1}
      />
      <SlideTabs
        tabs={titlesWithIcons}
        size="medium"
        appearance="round"
        onChange={setActiveIndex2}
        activeIndex={activeIndex2}
      />
    </Flex>
  );
};

export const Small = () => {
  const [activeIndex1, setActiveIndex1] = useState(0);
  const [activeIndex2, setActiveIndex2] = useState(0);
  return (
    <Flex flexDirection="column" gap={3}>
      <SlideTabs
        tabs={[
          {
            key: 'alarm',
            icon: Icon.AlarmRing,
            ariaLabel: 'alarm',
            tooltip: {
              content: 'ring ring',
              position: 'bottom',
            },
          },
          {
            key: 'bots',
            icon: Icon.Bots,
            ariaLabel: 'bots',
            tooltip: {
              content: 'beep boop',
              position: 'bottom',
            },
          },
          {
            key: 'check',
            icon: Icon.Check,
            ariaLabel: 'check',
            tooltip: {
              content: 'Do or do not. There is no try.',
              position: 'right',
            },
          },
        ]}
        size="small"
        appearance="round"
        onChange={setActiveIndex1}
        activeIndex={activeIndex1}
        fitContent
      />
      <SlideTabs
        tabs={[
          { key: 'kraken', title: 'Kraken' },
          { key: 'chubacabra', title: 'Chubacabra' },
          { key: 'yeti', title: 'Yeti' },
        ]}
        size="small"
        appearance="round"
        onChange={setActiveIndex2}
        activeIndex={activeIndex2}
        fitContent
      />
      <SlideTabs
        tabs={titlesWithIcons}
        size="small"
        appearance="round"
        onChange={setActiveIndex1}
        activeIndex={activeIndex1}
        fitContent
      />
    </Flex>
  );
};

export const StatusIcons = () => {
  const [activeIndex, setActiveIndex] = useState(0);
  const tabs: TabSpec[] = [
    { key: 'warning', title: 'warning', status: { kind: 'warning' } },
    { key: 'danger', title: 'danger', status: { kind: 'danger' } },
    { key: 'neutral', title: 'neutral', status: { kind: 'neutral' } },
  ];
  return (
    <Flex flexDirection="column" gap={3}>
      <SlideTabs
        tabs={tabs}
        activeIndex={activeIndex}
        onChange={setActiveIndex}
        isProcessing={true}
      />
      <SlideTabs
        tabs={tabs}
        activeIndex={activeIndex}
        onChange={setActiveIndex}
      />
      <SlideTabs
        tabs={tabs}
        hideStatusIconOnActiveTab
        activeIndex={activeIndex}
        onChange={setActiveIndex}
      />
    </Flex>
  );
};

export const DisabledTab = () => {
  return (
    <SlideTabs
      tabs={threeSimpleTabs}
      onChange={() => null}
      activeIndex={1}
      disabled={true}
    />
  );
};
