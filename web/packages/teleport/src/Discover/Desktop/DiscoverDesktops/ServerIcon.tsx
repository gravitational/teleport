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
import styled, { keyframes } from 'styled-components';

const Server = styled.div`
  width: 80px;
  height: 16px;
  padding: 0 7px;
  box-sizing: border-box;
  background: ${({ theme }) =>
    theme.type === 'light' ? '#6c6c6c' : '#f2e9f7'};
  margin-bottom: 8px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  position: relative;
  z-index: 5;
`;

const ServerLights = styled.div`
  display: flex;
  align-items: center;
`;

const blink = keyframes`
  46% {
    opacity: 1;
  }

  50% {
    opacity: 0;
  }

  54% {
    opacity: 1;
  }
`;

const blink2 = keyframes`
  27% {
    opacity: 1;
  }
  
  30% {
    opacity: 0;
  }

  33% {
    opacity: 1;
  }
`;

const blink3 = keyframes`
  68% {
    opacity: 1;
  }

  70% {
    opacity: 0;
  }

  72% {
    opacity: 1;
  }
`;

const ServerLight = styled.div`
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 5px;
`;

const ServerLightGreen = styled(ServerLight)`
  background: #31c842;
`;

export const ServerLightGreen1 = styled(ServerLightGreen)`
  animation: ${blink} 8s step-start 0s infinite;
`;

export const ServerLightGreen2 = styled(ServerLightGreen)`
  animation: ${blink2} 10s step-start 0s infinite;
`;

export const ServerLightGreen3 = styled(ServerLightGreen)`
  animation: ${blink3} 12s step-start 0s infinite;
`;

const ServerLines = styled.div`
  display: flex;
  flex: 1;
  align-items: flex-end;
  flex-direction: column;
`;

const ServerLine = styled.div`
  height: 3px;
  border-radius: 5px;
  background: ${({ theme }) =>
    theme.type === 'light' ? 'rgba(255, 255, 255, 0.4)' : 'rgba(0, 0, 0, 0.4)'};
  margin-left: 5px;
  overflow: hidden;
`;

const ServerLinesTop = styled.div`
  display: flex;
  justify-content: space-between;
  margin-bottom: 2px;
`;

interface ServerIconProps {
  light: React.ReactNode;
}

export function ServerIcon(props: ServerIconProps) {
  return (
    <Server>
      <ServerLights>{props.light}</ServerLights>
      <ServerLines>
        <ServerLinesTop>
          <ServerLine style={{ width: 5 }} />
          <ServerLine style={{ width: 30 }} />
        </ServerLinesTop>
        <ServerLine style={{ width: 20 }} />
      </ServerLines>
    </Server>
  );
}
