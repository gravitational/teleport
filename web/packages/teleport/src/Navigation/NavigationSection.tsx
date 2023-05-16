/*
Copyright 2023 Gravitational, Inc.

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
import styled from 'styled-components';

import { NavigationItem } from 'teleport/Navigation/NavigationItem';
import { NavigationItemSize } from 'teleport/Navigation/common';

import type { TeleportFeature } from 'teleport/types';

interface NavigationSectionProps {
  title: string;
  items: TeleportFeature[];
  transitionDelay: number;
  transitionDelayPerItem: number;
  visible: boolean;
}

const Title = styled.h3`
  font-size: 13px;
  line-height: 14px;
  color: ${props => props.theme.colors.text.slightlyMuted};
  margin-left: 32px;
  transition: transform 0.3s cubic-bezier(0.19, 1, 0.22, 1),
    opacity 0.15s ease-in;
  will-change: transform;
  margin-top: 33px;

  &:first-of-type {
    margin-top: 13px;
  }
`;

export function NavigationSection(props: NavigationSectionProps) {
  const items = [];

  let transitionDelay = props.transitionDelay;
  for (const [index, item] of props.items.entries()) {
    transitionDelay += props.transitionDelayPerItem;

    items.push(
      <NavigationItem
        feature={item}
        key={index}
        size={NavigationItemSize.Small}
        transitionDelay={transitionDelay}
        visible={props.visible}
      />
    );
  }

  return (
    <>
      <Title
        style={{
          transitionDelay: `${props.transitionDelay}ms,0s`,
          transform: `translate3d(${
            props.visible ? 0 : 'calc(var(--sidebar-width) * -1)'
          }, 0, 0)`,
        }}
      >
        {props.title}
      </Title>

      {items}
    </>
  );
}
