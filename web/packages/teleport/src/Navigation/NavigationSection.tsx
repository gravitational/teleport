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
  transition:
    transform 0.3s cubic-bezier(0.19, 1, 0.22, 1),
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
