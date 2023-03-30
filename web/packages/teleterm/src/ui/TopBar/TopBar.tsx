/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import styled from 'styled-components';

import QuickInput from '../QuickInput';

import { Connections } from './Connections';
import { Clusters } from './Clusters';
import { Identity } from './Identity';
import { NavigationMenu } from './NavigationMenu';

export function TopBar() {
  return (
    <Grid>
      <JustifyLeft>
        <Connections />
      </JustifyLeft>
      <CentralContainer>
        <Clusters />
        <QuickInput />
      </CentralContainer>
      <JustifyRight>
        <NavigationMenu />
        <Identity />
      </JustifyRight>
    </Grid>
  );
}

const Grid = styled.div`
  background: ${props => props.theme.colors.levels.surfaceSecondary};
  display: grid;
  grid-template-columns: 1fr minmax(0, 700px) 1fr;
  width: 100%;
  padding: 8px 16px;
  height: 56px;
  box-sizing: border-box;
  align-items: center;
`;

const CentralContainer = styled.div`
  display: grid;
  column-gap: 12px;
  margin: auto 12px;
  grid-auto-flow: column;
  grid-auto-columns: 2fr 5fr; // 1fr for a single child, 2fr 5fr for two children
  align-items: center;
  height: 100%;
`;

const JustifyLeft = styled.div`
  display: flex;
  justify-self: start;
  align-items: center;
  height: 100%;
`;

const JustifyRight = styled.div`
  display: flex;
  justify-self: end;
  align-items: center;
  height: 100%;
`;
