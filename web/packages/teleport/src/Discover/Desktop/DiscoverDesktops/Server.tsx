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
