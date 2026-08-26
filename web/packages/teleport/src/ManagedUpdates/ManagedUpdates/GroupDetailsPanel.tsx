/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { Box, Flex, H2, Link, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { ArrowSquareOut } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import {
  RolloutGroupInfo,
  RolloutInfo,
} from 'teleport/services/managedUpdates';

import { InfoItem, StatusBadge, VersionTableContainer } from '../shared';
import {
  getInstanceInventoryUrlFilteredByGroup,
  getOrphanedCount,
  getProgress,
} from './utils';

interface GroupDetailsPanelProps {
  group: RolloutGroupInfo;
  rollout?: RolloutInfo;
  orphanedAgentVersionCounts?: Record<string, number>;
}

export function GroupDetailsPanel({
  group,
  rollout,
  orphanedAgentVersionCounts,
}: GroupDetailsPanelProps) {
  const percent = getProgress(group);
  const versionCounts = group.agentVersionCounts || {};
  const versions = new Set(Object.keys(versionCounts));
  // We always show the start and target versions as rows even if their counts are 0.
  if (rollout?.startVersion) versions.add(rollout.startVersion);
  if (rollout?.targetVersion) versions.add(rollout.targetVersion);
  const versionData = Array.from(versions).map(version => ({
    version,
    count: versionCounts[version] || 0,
    isStart: version === rollout?.startVersion,
    isTarget: version === rollout?.targetVersion,
  }));

  const orphanedCount = getOrphanedCount(orphanedAgentVersionCounts);
  const hasOrphanedAgents = orphanedCount > 0;
  const orphanedVersions = new Set(
    Object.keys(orphanedAgentVersionCounts || {})
  );
  if (rollout?.startVersion) orphanedVersions.add(rollout.startVersion);
  if (rollout?.targetVersion) orphanedVersions.add(rollout.targetVersion);
  const orphanedVersionData = hasOrphanedAgents
    ? Array.from(orphanedVersions).map(version => ({
        version,
        count: orphanedAgentVersionCounts?.[version] || 0,
        isStart: version === rollout?.startVersion,
        isTarget: version === rollout?.targetVersion,
      }))
    : [];
  const orphanedUpToDate =
    orphanedAgentVersionCounts?.[rollout?.targetVersion || ''] || 0;
  const orphanedPercent =
    orphanedCount > 0
      ? Math.round((orphanedUpToDate / orphanedCount) * 100)
      : 0;

  return (
    <Box>
      <Box mb={3} mt={3}>
        <H2>{group.name}</H2>
      </Box>

      <Box mb={4}>
        <InfoItem
          label="Status"
          value={<StatusBadge state={group.state} />}
          labelWidth={100}
          mb={2}
        />
        <InfoItem
          label="Progress"
          value={
            <>
              {percent}% complete{' '}
              <Text as="span" color="text.muted">
                ({group.upToDateCount} of {group.presentCount})
              </Text>
            </>
          }
          labelWidth={100}
          mb={2}
        />
        {rollout?.strategy === 'halt-on-error' && (
          <InfoItem
            label="Group Order"
            value={group.position || '-'}
            labelWidth={100}
            mb={2}
          />
        )}
        <InfoItem
          label="Group Count"
          value={
            <Flex flexDirection="column">
              <Flex alignItems="center" gap={1}>
                <Text>{group.presentCount} agent instances</Text>
                <HoverTooltip tipContent="View in instance inventory">
                  <Link
                    href={getInstanceInventoryUrlFilteredByGroup(group.name)}
                    onClick={e => e.stopPropagation()}
                    css={`
                      display: inline-flex;
                      align-items: center;
                    `}
                  >
                    <ArrowSquareOut size="small" color="text.muted" />
                  </Link>
                </HoverTooltip>
              </Flex>
              {group.initialCount > 0 && (
                <Text typography="body2" color="text.muted">
                  {group.initialCount} at start time
                </Text>
              )}
            </Flex>
          }
          labelWidth={100}
          mb={2}
        />
      </Box>

      <VersionTable data={versionData} totalCount={group.presentCount} />

      {hasOrphanedAgents && (
        <Box mt={4}>
          <Text typography="h3" mb={2} color="interactive.solid.alert.default">
            + {orphanedCount} ungrouped agent instances
          </Text>
          <Text typography="body2" color="text.muted" mb={3}>
            Ungrouped agent instances are agent instances not assigned to any
            rollout group defined in the rollout configuration. If detected,
            these ungrouped agent instances are automatically added to the last
            group. <br />
            <br />
            If this is unexpected, double check your cluster&apos;s rollout
            configuration.
          </Text>

          <Box mb={3}>
            <InfoItem
              label="Progress"
              value={
                <>
                  {orphanedPercent}% complete{' '}
                  <Text as="span" color="text.muted">
                    ({orphanedUpToDate} of {orphanedCount})
                  </Text>
                </>
              }
              labelWidth={100}
              mb={2}
            />
            <InfoItem
              label="Group Count"
              value={`${orphanedCount} agent instances`}
              labelWidth={100}
              mb={2}
            />
          </Box>

          <VersionTable data={orphanedVersionData} totalCount={orphanedCount} />
        </Box>
      )}
    </Box>
  );
}

interface VersionData {
  version: string;
  count: number;
  isStart: boolean;
  isTarget: boolean;
}

function VersionTable({
  data,
  totalCount,
}: {
  data: VersionData[];
  totalCount: number;
}) {
  return (
    <VersionTableContainer>
      <Table
        data={data}
        columns={[
          {
            key: 'version',
            headerText: 'Version',
            isSortable: true,
            render: item => (
              <Cell>
                <Flex alignItems="center" gap={1}>
                  <Text typography="body2">{item.version}</Text>
                  {item.isStart && (
                    <Text color="text.muted" typography="body2">
                      (Start)
                    </Text>
                  )}
                  {item.isTarget && (
                    <Text color="text.muted" typography="body2">
                      (Target)
                    </Text>
                  )}
                </Flex>
              </Cell>
            ),
          },
          {
            key: 'count',
            headerText: 'Instances Updated',
            isSortable: true,
            render: item => {
              const pct =
                totalCount > 0
                  ? Math.round((item.count / totalCount) * 100)
                  : 0;
              return (
                <Cell>
                  <Flex alignItems="center" gap={2}>
                    <Text typography="body2">{item.count}</Text>
                    <Text typography="body2" color="text.muted">
                      ({pct}%)
                    </Text>
                  </Flex>
                </Cell>
              );
            },
          },
        ]}
        emptyText="No version data found"
        initialSort={{ key: 'version', dir: 'ASC' }}
      />
    </VersionTableContainer>
  );
}
