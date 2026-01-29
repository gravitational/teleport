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

import { Alert, Box, ButtonIcon, Flex, Text } from 'design';
import { Info, Refresh } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import { capitalizeFirstLetter } from 'shared/utils/text';

import {
  GroupAction,
  RolloutGroupInfo,
  RolloutInfo,
  ToolsAutoUpdateInfo,
} from 'teleport/services/managedUpdates';

import {
  Card,
  CardTitle,
  DOCS_URL,
  DocsLink,
  InfoItem,
  NoPermissionCardContent,
  NotConfiguredText,
  TOOLS_DOCS_URL,
} from '../shared';
import { GroupsTable } from './GroupsTable';
import { getOrphanedCount, isModeEnabled } from './utils';

interface ClientToolsCardProps {
  tools?: ToolsAutoUpdateInfo;
  fullWidth?: boolean;
  hasPermission?: boolean;
}

export function ClientToolsCard({
  tools,
  fullWidth,
  hasPermission = true,
}: ClientToolsCardProps) {
  const isToolsConfigured = isModeEnabled(tools?.mode);

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

interface RolloutCardProps {
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
}

export function RolloutCard({
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
}: RolloutCardProps) {
  const isRolloutConfigured = isModeEnabled(rollout?.mode);
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
            {!isImmediateSchedule && (
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
            )}
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
