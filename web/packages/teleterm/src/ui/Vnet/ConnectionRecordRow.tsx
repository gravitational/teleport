/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Flex, Text } from 'design';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import {
  ConnectionRecord,
  ConnectionRecordState,
} from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';

import { ConnectionStatusIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';

import { processDisplayName, useAppIcon } from './clientProcess';
import { formatBytes } from './ConnectionStatRow';

/**
 * ConnectionRecordRow shows a single connection made through VNet: the program
 * that opened it, how much data it transferred, how long it took, and whether
 * it is still active, has ended, or failed to be established.
 */
export const ConnectionRecordRow = (props: { record: ConnectionRecord }) => {
  const {
    clientProcessPath,
    localPort,
    startedAt,
    endedAt,
    bytesTx,
    bytesRx,
    state,
    errorMessage,
  } = props.record;

  const openedBy = processDisplayName(clientProcessPath) || 'Unknown program';
  const appIcon = useAppIcon(clientProcessPath);
  const startedAtDate = startedAt && Timestamp.toDate(startedAt);
  const isFailed = state === ConnectionRecordState.FAILED;

  return (
    <Flex flexDirection="column" gap={1} minWidth={0}>
      <Flex alignItems="center" gap={2} minWidth={0}>
        <ConnectionStatusIndicator
          status={connectionStatus(state)}
          title={stateLabel(state)}
        />
        {appIcon && (
          <img
            src={appIcon}
            alt=""
            css={`
              max-height: 16px;
              object-fit: contain;
            `}
          />
        )}
        <Text
          typography="body3"
          title={clientProcessPath}
          css={`
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          `}
        >
          {openedBy}
        </Text>

        {/*
          The byte counts are always shown, also for a failed connection: a
          connection that was established but transferred nothing back is a
          strong hint that the target itself is unreachable beyond VNet.
        */}
        <Text
          typography="body3"
          color="text.muted"
          css={`
            flex: 1;
            white-space: nowrap;
          `}
        >
          ↓ {formatBytes(bytesRx)} ↑ {formatBytes(bytesTx)}
        </Text>

        <Text
          typography="body3"
          color="text.muted"
          title={
            startedAtDate &&
            `Started at ${startedAtDate.toLocaleString()}, dialed port ${localPort}`
          }
          css={`
            white-space: nowrap;
          `}
        >
          Active for {formatDuration(startedAt, endedAt)}
        </Text>
      </Flex>

      {errorMessage && (
        <Text
          typography="body3"
          color={isFailed ? 'error.main' : 'text.muted'}
          title={errorMessage}
          ml={6}
          css={`
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          `}
        >
          {errorMessage}
        </Text>
      )}
    </Flex>
  );
};

function connectionStatus(
  state: ConnectionRecordState
): 'on' | 'error' | 'off' {
  switch (state) {
    case ConnectionRecordState.ACTIVE:
      return 'on';
    case ConnectionRecordState.FAILED:
      return 'error';
    default:
      return 'off';
  }
}

function stateLabel(state: ConnectionRecordState): string {
  switch (state) {
    case ConnectionRecordState.ACTIVE:
      return 'Active';
    case ConnectionRecordState.FAILED:
      return 'Failed to connect';
    case ConnectionRecordState.DONE:
      return 'Ended';
    default:
      return '';
  }
}

/**
 * formatDuration renders how long a connection lasted in a compact form such as
 * "30ms", "2.4s", or "5m 12s". A connection that is still active has no end time
 * yet; its duration is measured up to now, which stays fresh because the
 * connection list re-renders on every update of the connections stream.
 */
function formatDuration(
  startedAt: Timestamp | undefined,
  endedAt: Timestamp | undefined
): string {
  if (!startedAt) {
    return '';
  }
  const start = Timestamp.toDate(startedAt).getTime();
  const end = endedAt ? Timestamp.toDate(endedAt).getTime() : Date.now();
  const millis = Math.max(0, end - start);

  if (millis < 1000) {
    return `${millis}ms`;
  }
  const seconds = millis / 1000;
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m ${Math.floor(seconds % 60)}s`;
  }
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}m`;
}
