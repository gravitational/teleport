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

import { DesktopService } from 'teleport/Discover/Desktop/DiscoverDesktops/DesktopService';
import { WindowsDesktopService } from 'teleport/services/desktops';

import {
  AnimatedStyledSVG,
  StyledSVG,
} from 'teleport/Discover/Desktop/DiscoverDesktops/NodeLine';

import { ProxyServerIcon } from './Server';

const NodeHostname = styled.div`
  font-family:
    Menlo,
    DejaVu Sans Mono,
    Consolas,
    Lucida Console,
    monospace;
  font-size: 12px;
  color: ${props => props.theme.colors.text.primary};
  max-width: 184px;
  overflow-wrap: break-word;
`;

const NodeTitle = styled.div`
  font-size: 16px;
`;

const NodeIcon = styled.div`
  height: 92px;
  margin-bottom: 15px;
`;

const Nodes = styled.div`
  display: inline-flex;
  position: relative;
`;

const NodeLineContainer = styled.div`
  position: absolute;
  height: 94px;
  width: 257px;
  top: 0;
  left: 125px;
  right: 121px;
  border-bottom-left-radius: 5px;
  border-bottom-right-radius: 5px;
  overflow: hidden;
`;

function NodeLine() {
  return (
    <NodeLineContainer>
      <StyledSVG width={254} height={94} viewBox="0 0 254 93.5">
        <path d="M1.5,0V76.74c0,8.43,7.62,15.26,17.02,15.26H235.48c9.4,0,17.02-6.83,17.02-15.26V32.42" />
      </StyledSVG>
      <AnimatedStyledSVG width={254} height={94} viewBox="0 0 254 93.5">
        <path d="M1.5,0V76.74c0,8.43,7.62,15.26,17.02,15.26H235.48c9.4,0,17.02-6.83,17.02-15.26V32.42" />
      </AnimatedStyledSVG>
    </NodeLineContainer>
  );
}

const Node = styled.div`
  width: 250px;
  display: flex;
  align-items: center;
  flex-direction: column;
`;

function getProxyAddress() {
  const { hostname, port } = window.location;

  if (port === '443' || !port) {
    return hostname;
  }

  return `${hostname}:${port}`;
}

interface ProxyDesktopServiceDiagramProps {
  result: WindowsDesktopService;
  desktopServiceRef: React.Ref<HTMLDivElement>;
}

export function ProxyDesktopServiceDiagram(
  props: ProxyDesktopServiceDiagramProps
) {
  const proxyAddress = getProxyAddress();

  return (
    <div>
      <Nodes>
        <NodeLine />
        <Node>
          <NodeIcon>
            <ProxyServerIcon />
          </NodeIcon>

          <NodeTitle>Teleport Proxy</NodeTitle>
          <NodeHostname>{proxyAddress}</NodeHostname>
        </Node>

        <Node>
          <NodeIcon>
            <DesktopService desktopServiceRef={props.desktopServiceRef} />
          </NodeIcon>

          <NodeTitle>Desktop Service</NodeTitle>
          <NodeHostname>{props.result && props.result.hostname}</NodeHostname>
        </Node>
      </Nodes>
    </div>
  );
}
