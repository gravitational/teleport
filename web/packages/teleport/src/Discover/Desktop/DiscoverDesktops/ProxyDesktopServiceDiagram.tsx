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
