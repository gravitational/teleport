/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { type JSX } from 'react';
import styled from 'styled-components';

import { Flex, H2, Text } from 'design';
import * as Icons from 'design/Icon';
import { TeleportGearIcon } from 'design/SVGIcon';
import { MenuIcon } from 'shared/components/MenuAction';

export const WARN_THRESHOLD = 150;
export const ERROR_THRESHOLD = 400;

export enum LatencyColor {
  Ok = 'dataVisualisation.tertiary.caribbean',
  Warn = 'dataVisualisation.tertiary.abbey',
  Error = 'dataVisualisation.tertiary.sunflower',
  Unknown = 'text.muted',
}

export interface Latency {
  client: number;
  server: number;
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
export function latencyColors(latency: Latency | undefined): {
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
  latency: Latency | undefined;
}) {
  const colors = latencyColors(latency);

  return (
    <MenuIcon
      Icon={Icons.Wifi}
      tooltip="Network Connection"
      buttonIconProps={{ color: colors.total }}
    >
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
