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
import {
  ConnectionStat,
  RecentConnectionKind,
} from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';
import { pluralize } from 'shared/utils/text';

import { ConnectionKindIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionItem';

export const ConnectionStatRow = (props: { stat: ConnectionStat }) => {
  const {
    kind,
    displayName,
    port,
    successfulConnections,
    failedConnections,
    bytesTx,
    bytesRx,
    bytesTxPerSec,
    bytesRxPerSec,
  } = props.stat;
  // The port is only set for multi-port TCP apps.
  const name = port ? `${displayName}:${port}` : displayName;

  return (
    <Flex alignItems="center" gap={1} minWidth={0}>
      <ConnectionKindIndicator
        css={`
          padding: 3px 6px;
          font-weight: 600;
        `}
      >
        {kindLabel(kind)}
      </ConnectionKindIndicator>
      <Flex flexDirection="column" flex="1" minWidth={0}>
        <Text
          typography="body2"
          title={name}
          mt={1}
          css={`
            line-height: normal;
            min-width: 0;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
          `}
        >
          {name}
        </Text>
        <Flex alignItems="center" gap={1} minWidth={0}>
          <Text
            typography="body3"
            color="text.muted"
            title="Successful connections"
            css={`
              flex-shrink: 0;
              white-space: nowrap;
            `}
          >
            {successfulConnections.toString()}{' '}
            {pluralize(Number(successfulConnections), 'connection')}
          </Text>
          {failedConnections > 0n && (
            <Text
              typography="body3"
              color="error.main"
              title="Failed connections"
              css={`
                flex-shrink: 0;
                white-space: nowrap;
              `}
            >
              · ✕ {failedConnections.toString()}
            </Text>
          )}
          <Text
            typography="body3"
            color="text.muted"
            css={`
              min-width: 0;
              overflow: hidden;
              text-overflow: ellipsis;
              white-space: nowrap;
            `}
          >
            · ↓ {formatBytes(bytesRx)} · ↑ {formatBytes(bytesTx)}
            {` · ↓ ${formatBytes(bytesRxPerSec)}/s · ↑ ${formatBytes(bytesTxPerSec)}/s`}
          </Text>
        </Flex>
      </Flex>
    </Flex>
  );
};

export function kindLabel(kind: RecentConnectionKind): string {
  switch (kind) {
    case RecentConnectionKind.APP:
      return 'TCP';
    case RecentConnectionKind.SSH:
      return 'SSH';
    case RecentConnectionKind.DATABASE:
      return 'DB';
    default:
      return '';
  }
}

/** formatBytes formats a byte count into a human-readable string. */
export function formatBytes(bytes: bigint): string {
  if (bytes < 1024n) {
    return `${bytes} B`;
  }
  let value = Number(bytes);
  let unit = 'B';
  for (const nextUnit of ['KB', 'MB', 'GB', 'TB', 'PB']) {
    if (value < 1024) {
      break;
    }
    value /= 1024;
    unit = nextUnit;
  }
  return `${value.toFixed(1)} ${unit}`;
}
