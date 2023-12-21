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

import {
  ServerIcon,
  ServerLightGreen1,
  ServerLightGreen2,
  ServerLightGreen3,
} from 'teleport/Discover/Desktop/DiscoverDesktops/ServerIcon';

const Container = styled.div`
  display: flex;
  flex-direction: column;
  position: relative;
  padding-bottom: 10px;
`;

export function ProxyServerIcon() {
  return (
    <Container>
      <ServerIcon light={<ServerLightGreen1 />} />
      <ServerIcon light={<ServerLightGreen2 />} />
      <ServerIcon light={<ServerLightGreen3 />} />
    </Container>
  );
}
