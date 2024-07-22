/**
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

import styled from 'styled-components';
import React from 'react';

import * as Icons from 'design/Icon';
import { Flex, H2, Text } from 'design';
import { TeleportGearIcon } from 'design/SVGIcon';

import { DocumentSsh } from 'teleport/Console/stores';

import { MenuIcon } from 'shared/components/MenuAction';

export const WARN_THRESHOLD = 150;
export const ERROR_THRESHOLD = 400;

export enum LatencyColor {
  Ok = 'dataVisualisation.tertiary.caribbean',
  Warn = 'dataVisualisation.tertiary.abbey',
  Error = 'dataVisualisation.tertiary.sunflower',
  Unknown = 'text.muted',
}

function colorForLatency(l: number): LatencyColor {
  if (l >= ERROR_THRESHOLD) {
    return LatencyColor.Error;
  }

  if (l >= WARN_THRESHOLD) {
    return LatencyColor.Warn;
  }

  return LatencyColor.Ok;
}

// latencyColors determines the color to use for each leg of the connection
// and the total measurement.
export function latencyColors(latency: { client: number; server: number }): {
  client: LatencyColor;
  server: LatencyColor;
  total: LatencyColor;
} {
  if (latency === undefined) {
    return {
      client: LatencyColor.Unknown,
      server: LatencyColor.Unknown,
      total: LatencyColor.Unknown,
    };
  }

  const clientColor = colorForLatency(latency.client);
  const serverColor = colorForLatency(latency.server);

  // any + red = red
  if (latency.client >= ERROR_THRESHOLD || latency.server >= ERROR_THRESHOLD) {
    return {
      client: clientColor,
      server: serverColor,
      total: LatencyColor.Error,
    };
  }

  // any + yellow = yellow
  if (latency.client >= WARN_THRESHOLD || latency.server >= WARN_THRESHOLD) {
    return {
      client: clientColor,
      server: serverColor,
      total: LatencyColor.Warn,
    };
  }

  // green + green = green
  return { client: clientColor, server: serverColor, total: LatencyColor.Ok };
}

export function LatencyDiagnostic({
  latency,
}: {
  latency: DocumentSsh['latency'];
}) {
  const colors = latencyColors(latency);

  return (
    <MenuIcon Icon={Icons.Wifi} buttonIconProps={{ color: colors.total }}>
      <Container>
        <Flex gap={5} flexDirection="column">
          <H2>Network Connection</H2>

          <Flex alignItems="center">
            <IconContainer
              icon={<Icons.User />}
              text="You"
              alignItems="flex-start"
            />
            <Leg color={colors.client} latency={latency?.client} />
            <IconContainer
              icon={<TeleportGearIcon size={24} />}
              text="Teleport"
              alignItems="center"
            />
            <Leg color={colors.server} latency={latency?.server} />
            <IconContainer
              icon={<Icons.Server />}
              text="Server"
              alignItems="flex-end"
            />
          </Flex>

          <Flex flexDirection="column" alignItems="center">
            <Flex gap={1} flexDirection="row" alignItems="center">
              {latency === undefined && (
                <Text
                  italic
                  fontSize={2}
                  textAlign="center"
                  color={colors.total}
                >
                  Connecting
                </Text>
              )}

              {latency !== undefined && colors.total === LatencyColor.Error && (
                <Icons.WarningCircle size={20} color={colors.total} />
              )}

              {latency !== undefined && colors.total === LatencyColor.Warn && (
                <Icons.Warning size={20} color={colors.total} />
              )}

              {latency !== undefined && colors.total === LatencyColor.Ok && (
                <Icons.CircleCheck size={20} color={colors.total} />
              )}

              {latency !== undefined && (
                <Text bold fontSize={2} textAlign="center" color={colors.total}>
                  Total Latency: {latency.client + latency.server}ms
                </Text>
              )}
            </Flex>
          </Flex>
        </Flex>
      </Container>
    </MenuIcon>
  );
}

const IconContainer: React.FC<{
  icon: JSX.Element;
  text: string;
  alignItems: 'flex-start' | 'center' | 'flex-end';
}> = ({ icon, text, alignItems }) => (
  <Flex
    gap={1}
    width="24px"
    flexDirection="column"
    alignItems={alignItems}
    css={`
      flex-grow: 0;
    `}
  >
    {icon}
    <Text>{text}</Text>
  </Flex>
);

const Leg: React.FC<{
  color: LatencyColor;
  latency: number | undefined;
}> = ({ color, latency }) => (
  <Flex
    gap={1}
    flexDirection="column"
    alignItems="center"
    css={`
      flex-grow: 1;
      position: relative;
    `}
  >
    <DoubleSidedArrow />
    {latency !== undefined && <Text color={color}>{latency}ms</Text>}
    {latency === undefined && <Placeholder />}
  </Flex>
);

// Looks like `<----->`
const DoubleSidedArrow = () => {
  return (
    <Flex
      gap={1}
      flexDirection="row"
      alignItems="center"
      width="100%"
      p="0 16px"
    >
      <Icons.ChevronLeft
        size="small"
        color="text.muted"
        css={`
          left: 8px;
          position: absolute;
        `}
      />
      <Line />
      <Icons.ChevronRight
        size="small"
        color="text.muted"
        css={`
          right: 8px;
          position: absolute;
        `}
      />
    </Flex>
  );
};

const Container = styled.div`
  background: ${props => props.theme.colors.levels.elevated};
  padding: ${props => props.theme.space[4]}px;
  width: 370px;
`;

const Line = styled.div`
  color: ${props => props.theme.colors.text.muted};
  border: 0.5px dashed;
  width: 100%;
`;

const Placeholder = styled.div`
  height: 24px;
`;
