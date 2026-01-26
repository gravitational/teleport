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
import { useCallback, useEffect, useState } from 'react';
import { useTheme } from 'styled-components';

import { Alert, Box, ButtonIcon, Flex, Indicator, Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { Clock, Info, Refresh, Warning } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { MenuButton, MenuItem } from 'shared/components/MenuAction';
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';
import useAttempt from 'shared/hooks/useAttemptNext';
import { useInterval } from 'shared/hooks/useInterval';
import { capitalizeFirstLetter } from 'shared/utils/text';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import api from 'teleport/services/api';
import {
  ClusterMaintenanceInfo,
  GroupAction,
  GroupActionResponse,
  ManagedUpdatesDetails,
  RolloutGroupInfo,
  RolloutInfo,
  RolloutStrategy,
  ToolsAutoUpdateInfo,
} from 'teleport/services/managedUpdates';
import useTeleport from 'teleport/useTeleport';

import { GroupDetailsPanel } from './GroupDetailsPanel';
import {
  Card,
  CardTitle,
  DOCS_URL,
  DocsLink,
  InfoItem,
  MissingPermissionsBanner,
  NoPermissionCardContent,
  NotConfiguredText,
  POLLING_INTERVAL_MS,
  ProgressBar,
  StatusBadge,
  SUPPORT_URL,
  TableContainer,
  TOOLS_DOCS_URL,
} from './shared';
import {
  checkIsConfigured,
  getMissingPermissions,
  getOrphanedCount,
  getProgress,
  getReadableStateReason,
  getStateOrder,
} from './utils';

export interface ClusterMaintenanceCardProps {
  data: ClusterMaintenanceInfo;
}

export interface ManagedUpdatesProps {
  /**
   * Cluster maintenance card component. This is used by Cloud.
   */
  ClusterMaintenanceCard?: React.ComponentType<ClusterMaintenanceCardProps>;
}

export function ManagedUpdates({
  ClusterMaintenanceCard,
}: ManagedUpdatesProps) {
  const ctx = useTeleport();
  const acl = ctx.storeUser.state.acl;

  const canReadConfig = acl.autoUpdateConfig.read;
  const canReadVersion = acl.autoUpdateVersion.read;
  const canReadRollout = acl.autoUpdateAgentRollout.read;
  const canUpdateRollout = acl.autoUpdateAgentRollout.edit;

  const canViewAnything = canReadConfig || canReadVersion || canReadRollout;
  const canViewTools = canReadConfig && canReadVersion;
  const canViewRollout = canReadRollout;

  const missingPermissions = getMissingPermissions({
    canReadConfig,
    canReadVersion,
    canReadRollout,
  });

  const [data, setData] = useState<ManagedUpdatesDetails>(null);
  const [selectedGroupName, setSelectedGroupName] = useState<string>(null);
  const [lastSyncedTime, setLastSyncedTime] = useState<Date>(null);
  const [actionError, setActionError] = useState<string>(null);
  const { attempt, run } = useAttempt('processing');
  const { setInfoGuideConfig } = useInfoGuide();

  const selectedGroup =
    data?.groups?.find(g => g.name === selectedGroupName) || null;

  const fetchData = useCallback(() => {
    return api.get(cfg.getManagedUpdatesUrl()).then(response => {
      setData(response);
      setLastSyncedTime(new Date());
    });
  }, []);

  useEffect(() => {
    if (canViewAnything) {
      run(() => fetchData());
    }
  }, [run, fetchData, canViewAnything]);

  // Automatically re-sync every 1 minute
  useInterval(canViewAnything ? fetchData : null, POLLING_INTERVAL_MS);

  useEffect(() => {
    if (selectedGroup && data?.rollout) {
      setInfoGuideConfig({
        title: 'Progress Details',
        guide: (
          <GroupDetailsPanel
            group={selectedGroup}
            rollout={data.rollout}
            orphanedAgentVersionCounts={
              selectedGroup.isCatchAll
                ? data.orphanedAgentVersionCounts
                : undefined
            }
          />
        ),
        id: selectedGroup.name,
        panelWidth: 350,
        onClose: () => setSelectedGroupName(null),
      });
    } else {
      setInfoGuideConfig(null);
    }
  }, [
    selectedGroup,
    data?.rollout,
    data?.orphanedAgentVersionCounts,
    setInfoGuideConfig,
  ]);

  const handleGroupAction = async (
    action: GroupAction,
    groupName: string,
    force?: boolean
  ) => {
    setActionError(null);
    try {
      const url = cfg.getManagedUpdatesGroupActionUrl(groupName, action);
      const body = action === 'start' ? { force: force ?? false } : {};
      const response: GroupActionResponse = await api.post(url, body);

      // Update with the data that was returned
      if (response.group && data?.groups) {
        setData({
          ...data,
          groups: data.groups.map(g =>
            g.name === groupName ? response.group : g
          ),
        });
      }
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to perform group action'
      );
    }
  };

  if (!canViewAnything) {
    return (
      <FeatureBox px={9}>
        <FeatureHeader>
          <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
        </FeatureHeader>
        <MissingPermissionsBanner missingPermissions={missingPermissions} />
        <Box>
          <Box mb={3}>
            <ClientToolsCard fullWidth hasPermission={false} />
          </Box>
          <RolloutCard
            selectedGroupName={null}
            onGroupSelect={() => {}}
            onGroupAction={async () => {}}
            onRefresh={() => {}}
            lastSyncedTime={null}
            actionError={null}
            onDismissError={() => {}}
            canUpdateRollout={false}
            hasPermission={false}
          />
        </Box>
      </FeatureBox>
    );
  }

  if (attempt.status === 'processing') {
    return (
      <FeatureBox px={9}>
        <FeatureHeader>
          <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
        </FeatureHeader>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </FeatureBox>
    );
  }

  if (attempt.status === 'failed') {
    return (
      <FeatureBox px={9}>
        <FeatureHeader>
          <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
        </FeatureHeader>
        <Alert kind="danger" details={attempt.statusText}>
          Failed to load managed updates details
        </Alert>
      </FeatureBox>
    );
  }

  const isConfigured = checkIsConfigured(data);
  const hasPartialPermissions = missingPermissions.length > 0;

  return (
    <FeatureBox px={9}>
      <FeatureHeader>
        <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
      </FeatureHeader>

      {hasPartialPermissions && (
        <MissingPermissionsBanner missingPermissions={missingPermissions} />
      )}

      {!isConfigured && cfg.isCloud && (
        <Alert
          kind="warning"
          mb={3}
          primaryAction={{
            content: 'Go to Teleport Customer Center',
            href: SUPPORT_URL,
          }}
        >
          Could not detect configuration
          <Text typography="body2" mt={1}>
            Open a Support ticket in the Teleport Customer Center to report this
            view and request assistance for next steps.
          </Text>
        </Alert>
      )}

      <Box>
        <Flex gap={3} mb={3}>
          <ClientToolsCard
            tools={data?.tools}
            fullWidth={!data?.clusterMaintenance}
            hasPermission={canViewTools}
          />
          {data?.clusterMaintenance && ClusterMaintenanceCard && (
            <ClusterMaintenanceCard data={data.clusterMaintenance} />
          )}
        </Flex>

        <RolloutCard
          rollout={data?.rollout}
          groups={data?.groups}
          orphanedAgentVersionCounts={data?.orphanedAgentVersionCounts}
          hasPermission={canViewRollout}
          selectedGroupName={selectedGroupName}
          onGroupSelect={setSelectedGroupName}
          onGroupAction={handleGroupAction}
          onRefresh={fetchData}
          lastSyncedTime={lastSyncedTime}
          actionError={actionError}
          onDismissError={() => setActionError(null)}
          canUpdateRollout={canUpdateRollout}
        />
      </Box>
    </FeatureBox>
  );
}

function ClientToolsCard({
  tools,
  fullWidth,
  hasPermission = true,
}: {
  tools?: ToolsAutoUpdateInfo;
  fullWidth?: boolean;
  hasPermission?: boolean;
}) {
  const toolsMode = tools?.mode?.toLowerCase();
  const isToolsConfigured =
    !!toolsMode && toolsMode !== 'disabled' && toolsMode !== '';

  return (
    <Card flex={fullWidth ? 1 : '1 1 50%'}>
      <CardTitle>Client Tools Automatic Updates</CardTitle>
      <Flex alignItems="flex-start" gap={1} mb={3} flexDirection="column">
        <Text color="text.slightlyMuted">
          Keep client tools like <strong>tsh</strong> and <strong>tctl</strong>{' '}
          up to date with automatic or managed updates.
        </Text>
        <DocsLink docsUrl={TOOLS_DOCS_URL} />
      </Flex>
      <Box>
        {!hasPermission ? (
          <NoPermissionCardContent />
        ) : isToolsConfigured ? (
          <>
            <InfoItem
              label="Status"
              value={capitalizeFirstLetter(tools?.mode)}
            />
            <InfoItem
              label="Target version"
              value={tools?.targetVersion || '-'}
            />
          </>
        ) : (
          <NotConfiguredText docsUrl={TOOLS_DOCS_URL} />
        )}
      </Box>
    </Card>
  );
}

function RolloutCard({
  rollout,
  groups,
  orphanedAgentVersionCounts,
  selectedGroupName,
  onGroupSelect,
  onGroupAction,
  onRefresh,
  lastSyncedTime,
  actionError,
  onDismissError,
  canUpdateRollout,
  hasPermission = true,
}: {
  rollout?: RolloutInfo;
  groups?: RolloutGroupInfo[];
  orphanedAgentVersionCounts?: Record<string, number>;
  selectedGroupName: string;
  onGroupSelect: (name: string) => void;
  onGroupAction: (
    action: GroupAction,
    groupName: string,
    force?: boolean
  ) => Promise<void>;
  onRefresh: () => void;
  lastSyncedTime: Date;
  actionError: string;
  onDismissError: () => void;
  canUpdateRollout: boolean;
  hasPermission?: boolean;
}) {
  const rolloutMode = rollout?.mode?.toLowerCase();
  const isRolloutConfigured =
    !!rolloutMode && rolloutMode !== 'disabled' && rolloutMode !== '';
  const groupCount = groups?.length || 0;
  const isImmediateSchedule = rollout?.schedule === 'immediate';
  const orphanedCount = getOrphanedCount(orphanedAgentVersionCounts);
  const hasOrphanedAgents = orphanedCount > 0;
  const lastGroup = groups?.[groups.length - 1];

  return (
    <Card>
      <CardTitle>Rollout Configuration for Agent Instances</CardTitle>
      <Flex alignItems="flex-start" gap={1} mb={3} flexDirection="column">
        <Text color="text.slightlyMuted">
          Editors can set and manage rollout configuration in the CLI.
        </Text>
        <DocsLink docsUrl={DOCS_URL} />
      </Flex>

      {actionError && (
        <Alert kind="danger" mb={3} dismissible onDismiss={onDismissError}>
          {actionError}
        </Alert>
      )}

      {!hasPermission ? (
        <NoPermissionCardContent />
      ) : !isRolloutConfigured ? (
        <NotConfiguredText docsUrl={DOCS_URL} />
      ) : (
        <>
          <Box mb={3}>
            <InfoItem
              label="Status"
              value={capitalizeFirstLetter(rollout?.mode)}
            />
            <InfoItem label="Start" value={rollout?.startVersion || '-'} />
            <InfoItem label="Target" value={rollout?.targetVersion || '-'} />
            <InfoItem
              label="Strategy"
              value={
                <Flex alignItems="center" gap={1}>
                  {capitalizeFirstLetter(rollout?.strategy)}
                  <HoverTooltip
                    tipContent={
                      rollout?.strategy === 'halt-on-error'
                        ? 'Groups are updated sequentially. If a group fails, the rollout halts until manually resolved.'
                        : 'Groups are updated based on their configured maintenance window schedules.'
                    }
                    placement="right"
                  >
                    <Info size="small" color="text.muted" />
                  </HoverTooltip>
                </Flex>
              }
            />
          </Box>

          {isImmediateSchedule ? (
            <Alert kind="info" mb={0}>
              The rollout schedule has been set to <strong>immediate</strong>.
              Every group immediately updates to the target version.
            </Alert>
          ) : (
            <>
              {hasOrphanedAgents && lastGroup && (
                <Alert kind="warning" mb={3} dismissible>
                  Agent instances not assigned to a rollout group have been
                  detected.
                  <Text typography="body2" mt={1}>
                    Ungrouped agent instances can be reviewed in the progress
                    details of the last group <strong>{lastGroup.name}</strong>.
                    Ungrouped instances are listed separately and do not affect
                    the last group&apos;s rollout progress.
                  </Text>
                </Alert>
              )}

              <Flex justifyContent="space-between" alignItems="center" mb={3}>
                <Text typography="body2">
                  {groupCount} rollout group{groupCount !== 1 ? 's' : ''}
                </Text>
                <Flex alignItems="center" gap={2}>
                  {lastSyncedTime && (
                    <Text color="text.muted" typography="body3">
                      Last synced: {lastSyncedTime.toLocaleTimeString()}
                    </Text>
                  )}
                  <HoverTooltip tipContent="Refresh data">
                    <ButtonIcon
                      size={0}
                      onClick={onRefresh}
                      color="text.muted"
                      css={`
                        border-radius: ${p => p.theme.radii[2]}px;
                        border: 1px solid
                          ${p => p.theme.colors.interactive.tonal.neutral[0]};
                      `}
                    >
                      <Refresh size="small" />
                    </ButtonIcon>
                  </HoverTooltip>
                </Flex>
              </Flex>

              <GroupsTable
                groups={groups || []}
                orphanedCount={orphanedCount}
                selectedGroupName={selectedGroupName}
                onGroupSelect={onGroupSelect}
                onGroupAction={onGroupAction}
                strategy={rollout?.strategy}
                canUpdateRollout={canUpdateRollout}
              />
            </>
          )}
        </>
      )}
    </Card>
  );
}

function GroupsTable({
  groups,
  orphanedCount,
  selectedGroupName,
  onGroupSelect,
  onGroupAction,
  strategy,
  canUpdateRollout,
}: {
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
}) {
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
