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
