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

import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import { useEscape } from 'shared/hooks/useEscape';

import { Box } from 'design';

export type Positions = 'open' | 'closed';

type SlidePanelProps = {
  closePanel?: () => void;
  position: Positions;
};

type Props = PropsWithChildren<SlidePanelProps>;

export function SlidePanel({
  position,
  closePanel = () => {},
  children,
}: Props) {
  useEscape(() => closePanel());

  return (
    <>
      <Mask className={position} onClick={closePanel} data-testid="mask" />
      <Panel className={position} data-testid="panel">
        {children}
      </Panel>
    </>
  );
}

const Panel = styled(Box)`
  background: ${({ theme }) => theme.colors.levels.sunken};
  min-height: 100%;
  opacity: 1;
  padding: 20px;
  position: absolute;
  right: -500px;
  top: 0px;
  transition: right 500ms ease-out;
  width: 500px;
  z-index: 11;

  &.open {
    right: 0px;
  }
`;

const Mask = styled(Box)`
  background: #000;
  height: 100vh;
  left: 0;
  opacity: 0;
  position: absolute;
  top: 0;
  transition: opacity 500ms ease-out;
  width: 100vw;
  z-index: 10;
  pointer-events: none;

  &.open {
    opacity: 0.5;
    pointer-events: all;
  }
`;
