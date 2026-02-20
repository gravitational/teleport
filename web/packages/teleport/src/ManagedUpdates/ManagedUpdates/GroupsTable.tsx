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

import { format, formatDistanceToNowStrict } from 'date-fns';
import { useState } from 'react';
import { useTheme } from 'styled-components';

import { Flex, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { Clock, Warning } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';

import {
  GroupAction,
  RolloutGroupInfo,
  RolloutStrategy,
} from 'teleport/services/managedUpdates';

import { ProgressBar, StatusBadge, TableContainer } from '../shared';
import { getProgress, getReadableStateReason, getStateOrder } from './utils';

interface GroupsTableProps {
  groups: RolloutGroupInfo[];
  orphanedCount: number;
  selectedGroupName: string;
  onGroupSelect: (name: string) => void;
  onGroupAction: (
    action: GroupAction,
    groupName: string,
    force?: boolean
  ) => Promise<void>;
  strategy?: RolloutStrategy;
  canUpdateRollout: boolean;
}

export function GroupsTable({
  groups,
  orphanedCount,
  selectedGroupName,
  onGroupSelect,
  onGroupAction,
  strategy,
  canUpdateRollout,
}: GroupsTableProps) {
  const theme = useTheme();
  const [actionInProgress, setActionInProgress] = useState<string>(null);

  const handleAction = async (
    action: GroupAction,
    groupName: string,
    force?: boolean
  ) => {
    setActionInProgress(`${action}-${groupName}`);
    try {
      await onGroupAction(action, groupName, force);
    } finally {
      setActionInProgress(null);
    }
  };

  return (
    <TableContainer>
      <Table
        key={strategy}
        data={groups}
        columns={[
          // We only show the order column for halt-on-error strategy
          ...(strategy === 'halt-on-error'
            ? [
                {
                  key: 'position' as const,
                  headerText: 'Order',
                  isSortable: true,
                  render: (group: RolloutGroupInfo) => (
                    <Cell
                      css={`
                        text-align: center;
                        width: 60px;
                        padding-left: 0;
                        padding-right: 0;
                      `}
                    >
                      <Text typography="body2">{group.position || '-'}</Text>
                    </Cell>
                  ),
                },
              ]
            : []),
          {
            key: 'name',
            headerText: 'Rollout Group',
            isSortable: true,
            render: group => {
              const isLastWithOrphans = group.isCatchAll && orphanedCount > 0;
              return (
                <Cell>
                  <Text typography="body2" fontWeight={500}>
                    {group.name}
                  </Text>
                  <Text typography="body2" color="text.muted">
                    {group.presentCount} agent instances
                  </Text>
                  {isLastWithOrphans && (
                    <Text
                      typography="body2"
                      color="interactive.solid.alert.default"
                    >
                      + {orphanedCount} ungrouped instances
                    </Text>
                  )}
                </Cell>
              );
            },
          },
          {
            key: 'state',
            headerText: 'Status',
            isSortable: true,
            onSort: (a, b) => getStateOrder(a.state) - getStateOrder(b.state),
            render: group => (
              <Cell
                css={`
                  white-space: nowrap;
                `}
              >
                <StatusBadge state={group.state} />
              </Cell>
            ),
          },
          {
            key: 'upToDateCount',
            headerText: 'Progress',
            isSortable: true,
            onSort: (a, b) => getProgress(a) - getProgress(b),
            render: group => {
              const percent = getProgress(group);
              // If progress is >90%, but >10% of the initial agents have dropped, we show a warning that this group will never
              // complete automatically. This is because in the backend, we only mark a group as done when the `initialCount` of agents
              // is updated. This can never happen if agents drop or are deleted while the rollout is in progress.
              const hasAgentsDropped =
                percent > 90 && group.presentCount / group.initialCount < 0.9;

              return (
                <Cell
                  css={`
                    min-width: 180px;
                  `}
                >
                  <Flex flexDirection="column" gap={1}>
                    <Text typography="body2">
                      {percent}% complete{' '}
                      <Text as="span" color="text.muted" typography="body2">
                        ({group.upToDateCount} of {group.presentCount})
                      </Text>
                    </Text>
                    <ProgressBar percent={percent} />
                    {hasAgentsDropped && (
                      <HoverTooltip
                        tipContent={
                          'This group will not automatically complete because more than 10% of the initial agents in this group are no longer connected. For the rollout to proceed, the agents must be reconnected or you can manually mark this group as done.'
                        }
                        placement="top"
                      >
                        <Flex alignItems="flex-start" gap={1}>
                          <Warning
                            size="small"
                            color="warning.main"
                            mr={1}
                            css={`
                              margin-top: 2px;
                            `}
                          />
                          <Text typography="body3" color="warning.main">
                            Action required. Hover for details.
                          </Text>
                        </Flex>
                      </HoverTooltip>
                    )}
                  </Flex>
                </Cell>
              );
            },
          },
          {
            key: 'stateReason',
            headerText: 'Status Detail',
            isSortable: true,
            render: group => {
              const reason = getReadableStateReason(group.stateReason);
              const statusDetail =
                group.state === 'canary'
                  ? `Canary (${group.canarySuccessCount}/${group.canaryCount}) - ${reason}`
                  : reason;

              return (
                <Cell>
                  {statusDetail && (
                    <>
                      <Text typography="body2">{statusDetail}</Text>
                      {group.lastUpdateTime && (
                        <Flex alignItems="center" gap={1}>
                          <Text typography="body2" color="text.muted">
                            {formatDistanceToNowStrict(
                              new Date(group.lastUpdateTime),
                              { addSuffix: true }
                            )}
                          </Text>
                          <HoverTooltip
                            tipContent={format(
                              new Date(group.lastUpdateTime),
                              "MMMM d, yyyy 'at' h:mm a"
                            )}
                          >
                            <Clock size="small" color="text.muted" />
                          </HoverTooltip>
                        </Flex>
                      )}
                    </>
                  )}
                </Cell>
              );
            },
          },
          {
            key: 'startTime',
            headerText: 'Start Time',
            isSortable: true,
            render: group => (
              <Cell>
                <Text typography="body2" color="text.muted">
                  {group.startTime
                    ? format(
                        new Date(group.startTime),
                        "MMMM d, yyyy '·' h:mm a"
                      )
                    : 'Not started'}
                </Text>
              </Cell>
            ),
          },
          {
            altKey: 'actions',
            render: group => {
              const isLoading = actionInProgress !== null;
              const isDisabled = !canUpdateRollout || isLoading;

              const button = (
                <MenuButton
                  buttonText="Actions"
                  buttonProps={{
                    disabled: isDisabled,
                    style: isLoading ? { cursor: 'wait' } : undefined,
                  }}
                >
                  <MenuItem
                    onClick={() => handleAction('start', group.name)}
                    disabled={
                      isLoading ||
                      group.state === 'active' ||
                      group.state === 'done'
                    }
                  >
                    Start Update
                  </MenuItem>
                  <MenuItem
                    onClick={() => handleAction('start', group.name, true)}
                    disabled={
                      isLoading ||
                      group.state === 'active' ||
                      group.state === 'done'
                    }
                  >
                    Force Update
                  </MenuItem>
                  <MenuItem
                    onClick={() => handleAction('done', group.name)}
                    disabled={
                      isLoading ||
                      group.state === 'done' ||
                      group.state === 'unstarted'
                    }
                  >
                    Mark as done
                  </MenuItem>
                  <MenuItem
                    onClick={() => handleAction('rollback', group.name)}
                    disabled={
                      isLoading ||
                      group.state === 'rolledback' ||
                      group.state === 'unstarted'
                    }
                  >
                    Roll back
                  </MenuItem>
                </MenuButton>
              );

              return (
                <Cell align="right">
                  {!canUpdateRollout ? (
                    <HoverTooltip
                      tipContent={
                        <>
                          You need <code>update</code> permission for{' '}
                          <code>autoupdate_agent_rollout</code> to perform
                          actions on rollout groups.
                        </>
                      }
                    >
                      {button}
                    </HoverTooltip>
                  ) : (
                    button
                  )}
                </Cell>
              );
            },
          },
        ]}
        emptyText="No rollout groups configured"
        initialSort={{
          key: (strategy === 'halt-on-error'
            ? 'position'
            : 'upToDateCount') as keyof RolloutGroupInfo,
          dir: strategy === 'halt-on-error' ? 'ASC' : 'DESC',
        }}
        row={{
          onClick: group =>
            onGroupSelect(selectedGroupName === group.name ? null : group.name),
          getStyle: group =>
            selectedGroupName === group.name
              ? { backgroundColor: theme.colors.interactive.tonal.neutral[1] }
              : undefined,
        }}
      />
    </TableContainer>
  );
}
