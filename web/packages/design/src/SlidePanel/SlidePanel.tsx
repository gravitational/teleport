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
