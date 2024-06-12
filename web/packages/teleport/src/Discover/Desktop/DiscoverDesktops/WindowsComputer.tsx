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

import { Cross } from 'design/Icon';
import { Flex } from 'design';

import teleportAppIcon from './teleport-app-icon.svg';
import windowsIcon from './windows.svg';

interface WindowsComputerProps {
  computerName: string;
  os: string;
  osVersion: string;
  address: string;
}

const SuccessMessage = styled.div`
  color: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  color: #9cb974;
  font-size: 12px;
  font-weight: 500;
  margin-bottom: 5px;
`;

const SuccessTick = styled.span`
  margin-right: 5px;
  font-size: 14px;
  font-family:
    Menlo,
    DejaVu Sans Mono,
    Consolas,
    Lucida Console,
    monospace;
`;

const Application = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 7px;
  margin-left: 1px;
`;

const ActiveApplication = styled(Application)`
  background: #323436;
  box-sizing: border-box;
  border-bottom: 1px solid #92c7ef;
`;

const TeleportAppIcon = styled.div`
  background: url(${teleportAppIcon}) no-repeat;
  width: 15px;
  height: 13px;
  position: relative;
  top: 1px;
  background-size: contain;
`;

const Applications = styled.div`
  display: flex;
  height: 30px;
`;

const DesktopTitleBar = styled.div`
  background: #d9d9d9;
  font-size: 12px;
  display: flex;
  justify-content: space-between;
  padding: 5px 10px;
  line-height: 1;
  color: rgba(0, 0, 0, 0.8);
  border-top-left-radius: 5px;
  border-top-right-radius: 5px;
  align-items: center;
  font-weight: bold;
`;

const DesktopAppContent = styled.div`
  background: white;
  padding: 13px 10px 5px;
  font-size: 12px;
  line-height: 1;
`;

const WindowsIcon = styled.div`
  background: url(${windowsIcon}) no-repeat;
  width: 16px;
  height: 16px;
  background-size: contain;
  flex: 0 0 16px;
`;

const DesktopStartMenu = styled.div`
  background: #000000;
  display: flex;
  justify-content: space-between;
  border-bottom-left-radius: 5px;
  border-bottom-right-radius: 5px;
  padding-right: 5px;
  height: 30px;
  color: white;
  font-size: 10px;
  align-items: center;
`;

const Label = styled.div`
  color: rgba(0, 0, 0, 0.5);
  font-size: 12px;
  margin-bottom: 5px;
`;

const ComputerName = styled.div`
  color: black;
  font-weight: bold;
  font-size: 15px;
  margin: 10px 0;
`;

const ComputerOS = styled.div`
  color: black;
  font-size: 13px;
  display: flex;
  justify-content: space-between;
`;

const ComputerOSVersion = styled.div`
  margin-top: 3px;
  font-size: 11px;
  color: rgba(0, 0, 0, 0.6);
`;

const ComputerAddress = styled.div`
  font-family:
    Menlo,
    DejaVu Sans Mono,
    Consolas,
    Lucida Console,
    monospace;
  font-size: 13px;
  color: rgba(0, 0, 0, 0.8);
`;

const ComputerInfo = styled.div`
  font-size: 10px;
  margin-bottom: 10px;
`;

export function WindowsComputer(props: WindowsComputerProps) {
  return (
    <>
      <DesktopTitleBar>
        <div>Teleport</div>

        <Cross color="black" size="small" />
      </DesktopTitleBar>
      <DesktopAppContent>
        <SuccessMessage>
          <SuccessTick>{'✔'}</SuccessTick> Teleport found this Desktop
        </SuccessMessage>

        <ComputerName>{props.computerName}</ComputerName>

        <Flex justifyContent="space-between">
          <ComputerInfo>
            <Label>Operating System</Label>

            <ComputerOS>{props.os}</ComputerOS>
            <ComputerOSVersion>{props.osVersion}</ComputerOSVersion>
          </ComputerInfo>
          <ComputerInfo>
            <Flex flexDirection="column" alignItems="flex-end">
              <Label>Address</Label>

              <ComputerAddress>{props.address}</ComputerAddress>
            </Flex>
          </ComputerInfo>
        </Flex>
      </DesktopAppContent>
      <DesktopStartMenu>
        <Applications>
          <Application>
            <WindowsIcon />
          </Application>

          <ActiveApplication>
            <TeleportAppIcon />
          </ActiveApplication>
        </Applications>

        <div>{getTime()}</div>
      </DesktopStartMenu>
    </>
  );
}

function getTime() {
  const date = new Date();

  return `${date.getHours()}:${date.getMinutes().toString().padStart(2, '0')}`;
}
