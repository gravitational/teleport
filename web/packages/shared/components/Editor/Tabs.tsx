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

import styled from 'styled-components';

import * as Icons from 'design/Icon';

interface TabsProps {
  items: string[];
  activeIndex: number;
  onSelect: (index: number) => void;
}

export function Tabs(props: TabsProps) {
  const tabs = props.items.map((name, index) => (
    <Tab
      key={index}
      active={index === props.activeIndex}
      onClick={() => props.onSelect(index)}
    >
      <Icons.Code color="white" size="small" mr={2} />
      {name}
    </Tab>
  ));

  return <TabsContainer>{tabs}</TabsContainer>;
}

export const TabsContainer = styled.div`
  background: #0a102c;
  display: flex;
`;

export const Tab = styled.div<{ active: boolean }>`
  background: rgba(255, 255, 255, 0.1);
  padding: 8px 20px 10px 15px;
  cursor: pointer;
  position: relative;
  color: white;
  display: flex;
  align-items: center;

  &:after {
    content: '';
    position: absolute;
    bottom: 0;
    height: 2px;
    left: 0;
    right: 0;
    background: ${p =>
      p.active ? 'linear-gradient(to right, #ec008c, #fc6767)' : 'transparent'};
  }
`;
