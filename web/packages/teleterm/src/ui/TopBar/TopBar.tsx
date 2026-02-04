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

import { RefObject } from 'react';
import styled from 'styled-components';

import { Flex } from 'design';

import { SearchBar } from '../Search';
import { AdditionalActions } from './AdditionalActions';
import { Clusters } from './Clusters';
import { Connections } from './Connections';
import { Identity } from './Identity';

export function TopBar(props: {
  connectMyComputerRef: RefObject<HTMLDivElement>;
  accessRequestRef: RefObject<HTMLDivElement>;
}) {
  return (
    <Grid>
      <JustifyLeft>
        <Connections />
        <div ref={props.connectMyComputerRef} />
      </JustifyLeft>
      <CentralContainer>
        <Clusters />
        <SearchBar />
      </CentralContainer>
      <JustifyRight>
        <div
          css={`
            height: 100%;
          `}
          ref={props.accessRequestRef}
        />
        <AdditionalActions />
        <Identity />
      </JustifyRight>
    </Grid>
  );
}

const Grid = styled(Flex).attrs({ gap: 3, py: 2, px: 3 })`
  background: ${props => props.theme.colors.levels.surface};
  width: 100%;
  height: 56px;
  align-items: center;
  justify-content: space-between;
`;

const CentralContainer = styled(Flex).attrs({ gap: 3 })`
  flex: 1;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-width: 0;
  max-width: calc(${props => props.theme.space[10]}px * 9);
`;

// Reserve space for dynamic icons (Connect My Computer and access requests)
// to prevent layout shift. Side containers must be of equal width to keep the
// search bar input centered, so that it is completely hidden when the search
// is open.
const SIDE_CONTAINER_WIDTH = '128px';

const JustifyLeft = styled(Flex).attrs({ gap: 3 })`
  width: ${SIDE_CONTAINER_WIDTH};
  align-items: center;
  height: 100%;
`;

const JustifyRight = styled(Flex).attrs({ gap: 2 })`
  width: ${SIDE_CONTAINER_WIDTH};
  justify-content: end;
  align-items: center;
  height: 100%;
`;
